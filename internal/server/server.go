// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins"
	"github.com/circonus-labs/circonus-agent/internal/check"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-agent/internal/plugins"
	"github.com/circonus-labs/circonus-agent/internal/statsd"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

type httpServer struct {
	address *net.TCPAddr
	server  *http.Server
}

type socketServer struct {
	address  *net.UnixAddr
	listener *net.UnixListener
	server   *http.Server
}

type sslServer struct {
	server   *http.Server
	address  *net.TCPAddr
	certFile string
	keyFile  string
}

// Server defines the listening servers.
type Server struct {
	check      *check.Check
	group      *errgroup.Group
	builtins   *builtins.Builtins
	plugins    *plugins.Plugins
	statsdSvr  *statsd.Server
	svrHTTPS   *sslServer
	svrHTTP    []*httpServer
	svrSockets []*socketServer
	groupCtx   context.Context
	logger     zerolog.Logger
}

type previousMetrics struct {
	ts      time.Time
	metrics *cgm.Metrics
}

var (
	pluginPathRx    = regexp.MustCompile("^/(run(/[a-zA-Z0-9_-]*)?)?$")
	inventoryPathRx = regexp.MustCompile("^/inventory/?$")
	writePathRx     = regexp.MustCompile("^/write/[a-zA-Z0-9_-]+$")
	statsPathRx     = regexp.MustCompile("^/stats/?$")
	promPathRx      = regexp.MustCompile("^/prom/?$")
	lastMetrics     = &previousMetrics{}
	lastMetricsmu   sync.Mutex
)

// New creates a new instance of the listening servers.
func New(ctx context.Context, c *check.Check, b *builtins.Builtins, p *plugins.Plugins, ss *statsd.Server) (*Server, error) {
	g, gctx := errgroup.WithContext(ctx)
	s := Server{
		group:     g,
		groupCtx:  gctx,
		logger:    log.With().Str("pkg", "server").Logger(),
		builtins:  b,
		plugins:   p,
		statsdSvr: ss,
		check:     c,
	}

	// HTTP listener (1-n)
	{
		serverList := viper.GetStringSlice(config.KeyListen)
		if len(serverList) == 0 {
			serverList = []string{defaults.Listen}
		}
		for idx, addr := range serverList {
			ta, err := config.ParseListen(addr)
			if err != nil {
				s.logger.Error().Err(err).Int("id", idx).Str("addr", addr).Msg("resolving address")
				return nil, fmt.Errorf("HTTP Server: %w", err)
			}

			svr := httpServer{
				address: ta,
				server: &http.Server{
					Addr:    ta.String(),
					Handler: http.HandlerFunc(s.router),
				},
			}
			svr.server.SetKeepAlivesEnabled(false)

			s.svrHTTP = append(s.svrHTTP, &svr)
		}
	}

	// HTTPS listener (singular)
	if addr := viper.GetString(config.KeySSLListen); addr != "" {
		ta, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			s.logger.Error().Err(err).Str("addr", addr).Msg("resolving address")
			return nil, fmt.Errorf("SSL Server: %w", err)
		}

		certFile := viper.GetString(config.KeySSLCertFile)
		if _, err := os.Stat(certFile); os.IsNotExist(err) {
			s.logger.Error().Err(err).Str("cert_file", certFile).Msg("SSL server")
			return nil, fmt.Errorf("SSL server cert file: %w", err)
		}

		keyFile := viper.GetString(config.KeySSLKeyFile)
		if _, err := os.Stat(keyFile); os.IsNotExist(err) {
			s.logger.Error().Err(err).Str("key_file", keyFile).Msg("SSL server")
			return nil, fmt.Errorf("SSL server key file: %w", err)
		}

		svr := sslServer{
			address:  ta,
			certFile: certFile,
			keyFile:  keyFile,
			server: &http.Server{
				Addr:    ta.String(),
				Handler: http.HandlerFunc(s.router),
				// Handler: httpgzip.NewHandler(http.HandlerFunc(s.router), []string{"application/json"}),
			},
		}

		svr.server.SetKeepAlivesEnabled(false)
		s.svrHTTPS = &svr
	}

	// Socket listener (1-n)
	if runtime.GOOS != "windows" {
		socketList := viper.GetStringSlice(config.KeyListenSocket)
		for idx, addr := range socketList {
			ua, err := net.ResolveUnixAddr("unix", addr)
			if err != nil {
				s.logger.Error().Err(err).Int("id", idx).Str("addr", addr).Msg("resolving address")
				return nil, fmt.Errorf("socket server: %w", err)
			}

			if _, serr := os.Stat(ua.String()); serr == nil || !os.IsNotExist(serr) {
				s.logger.Error().Int("id", idx).Str("socket_file", ua.String()).Msg("already exists")
				return nil, fmt.Errorf("socket server file (%s) exists", ua.String()) //nolint:goerr113
			}

			ul, err := net.ListenUnix(ua.Network(), ua)
			if err != nil {
				s.logger.Error().Err(err).Int("id", idx).Str("addr", ua.String()).Msg("creating socket")
				return nil, fmt.Errorf("creating socket: %w", err)
			}

			s.svrSockets = append(s.svrSockets, &socketServer{
				address:  ua,
				listener: ul,
				server:   &http.Server{Handler: http.HandlerFunc(s.socketHandler)},
			})
		}
	}

	return &s, nil
}

// GetReverseAgentAddress returns the address reverse should use to talk to the agent.
// Initially, this is the first server address.
func (s *Server) GetReverseAgentAddress() (string, error) {
	if len(s.svrHTTP) == 0 {
		return "", fmt.Errorf("no listen servers defined") //nolint:goerr113
	}
	return s.svrHTTP[0].address.String(), nil
}

// Start main listening server(s).
func (s *Server) Start() error {
	if len(s.svrHTTP) == 0 && s.svrHTTPS == nil && len(s.svrSockets) > 0 {
		return fmt.Errorf("no servers defined") //nolint:goerr113
	}

	s.group.Go(s.startHTTPS)

	for _, svrHTTP := range s.svrHTTP {
		svr := svrHTTP
		s.group.Go(func() error {
			return s.startHTTP(svr)
		})
	}

	for _, svrSocket := range s.svrSockets {
		svr := svrSocket
		s.group.Go(func() error {
			return s.startSocket(svr)
		})
	}

	go func() {
		<-s.groupCtx.Done()
		s.Stop()
	}()

	return s.group.Wait() //nolint:wrapcheck
}

// Stop the servers in an orderly, graceful fashion.
func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, svrHTTP := range s.svrHTTP {
		s.logger.Info().Msg("stopping HTTP server")
		err := svrHTTP.server.Shutdown(ctx)
		if err != nil {
			s.logger.Warn().Err(err).Msg("closing HTTP server")
		}
	}

	if s.svrHTTPS != nil {
		s.logger.Info().Msg("stopping HTTPS server")
		err := s.svrHTTPS.server.Shutdown(ctx)
		if err != nil {
			s.logger.Warn().Err(err).Msg("closing HTTPS server")
		}
	}

	for _, svrSocket := range s.svrSockets {
		s.logger.Info().Str("server", svrSocket.address.Name).Msg("stopping Socket server")
		err := svrSocket.server.Shutdown(ctx)
		if err != nil {
			s.logger.Warn().Err(err).Str("server", svrSocket.address.Name).Msg("closing Socket server")
		}
	}
}

func (s *Server) startHTTP(svr *httpServer) error {
	if svr == nil {
		s.logger.Debug().Msg("no listen configured, skipping server")
		return nil
	}
	if svr.address == nil || svr.server == nil {
		s.logger.Debug().Msg("listen not configured, skipping server")
		return nil
	}

	s.logger.Info().Str("listen", svr.address.String()).Msg("Starting")
	if err := svr.server.ListenAndServe(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			s.logger.Fatal().Err(err).Msg("HTTP Server, stopping agent")
			return fmt.Errorf("HTTP server: %w", err)
		}

	}
	return nil
}

func (s *Server) startHTTPS() error {
	if s.svrHTTPS == nil {
		s.logger.Debug().Msg("no SSL listen configured, skipping server")
		return nil
	}
	s.logger.Info().Str("listen", s.svrHTTPS.server.Addr).Msg("SSL starting")
	if err := s.svrHTTPS.server.ListenAndServeTLS(s.svrHTTPS.certFile, s.svrHTTPS.keyFile); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			s.logger.Fatal().Err(err).Msg("SSL Server, stopping agent")
			return fmt.Errorf("SSL server: %w", err)
		}
	}
	return nil
}

func (s *Server) startSocket(svr *socketServer) error {
	if svr == nil {
		s.logger.Debug().Msg("no socket configured, skipping")
		return nil
	}
	if svr.address == nil || svr.listener == nil || svr.server == nil {
		s.logger.Debug().Msg("socket not configured, skipping")
		return nil
	}
	if runtime.GOOS == "windows" {
		s.logger.Warn().Msg("platform does not support unix sockets")
		return nil
	}

	s.logger.Info().Str("listen", svr.address.String()).Msg("Socket starting")
	if err := svr.server.Serve(svr.listener); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			s.logger.Fatal().Err(err).Str("socket", svr.address.String()).Msg("Socket Server, stopping agent")
			return fmt.Errorf("socket server: %w", err)
		}
	}
	return nil
}
