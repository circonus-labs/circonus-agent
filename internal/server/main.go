// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"context"
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
	"github.com/pkg/errors"
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
	address  *net.TCPAddr
	certFile string
	keyFile  string
	server   *http.Server
}

// Server defines the listening servers
type Server struct {
	group      *errgroup.Group
	groupCtx   context.Context
	builtins   *builtins.Builtins
	check      *check.Check
	ctx        context.Context
	logger     zerolog.Logger
	plugins    *plugins.Plugins
	svrHTTP    []*httpServer
	svrHTTPS   *sslServer
	svrSockets []*socketServer
	statsdSvr  *statsd.Server
}

type previousMetrics struct {
	metrics *cgm.Metrics
	ts      time.Time
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

// New creates a new instance of the listening servers
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
				return nil, errors.Wrap(err, "HTTP Server")
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
			return nil, errors.Wrap(err, "SSL Server")
		}

		certFile := viper.GetString(config.KeySSLCertFile)
		if _, err := os.Stat(certFile); os.IsNotExist(err) {
			s.logger.Error().Err(err).Str("cert_file", certFile).Msg("SSL server")
			return nil, errors.Wrapf(err, "SSL server cert file")
		}

		keyFile := viper.GetString(config.KeySSLKeyFile)
		if _, err := os.Stat(keyFile); os.IsNotExist(err) {
			s.logger.Error().Err(err).Str("key_file", keyFile).Msg("SSL server")
			return nil, errors.Wrapf(err, "SSL server key file")
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
				return nil, errors.Wrap(err, "Socket server")
			}

			if _, serr := os.Stat(ua.String()); serr == nil || !os.IsNotExist(serr) {
				s.logger.Error().Int("id", idx).Str("socket_file", ua.String()).Msg("already exists")
				return nil, errors.Errorf("Socket server file (%s) exists", ua.String())
			}

			ul, err := net.ListenUnix(ua.Network(), ua)
			if err != nil {
				s.logger.Error().Err(err).Int("id", idx).Str("addr", ua.String()).Msg("creating socket")
				return nil, errors.Wrap(err, "creating socket")
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
		return "", errors.New("No listen servers defined")
	}
	return s.svrHTTP[0].address.String(), nil
}

// Start main listening server(s)
func (s *Server) Start() error {
	if len(s.svrHTTP) == 0 && s.svrHTTPS == nil && len(s.svrSockets) > 0 {
		return errors.New("No servers defined")
	}

	s.group.Go(s.startHTTPS)

	for _, svrHTTP := range s.svrHTTP {
		s.group.Go(func() error {
			return s.startHTTP(svrHTTP)
		})
	}

	for _, svrSocket := range s.svrSockets {
		s.group.Go(func() error {
			return s.startSocket(svrSocket)
		})
	}

	go func() {
		select {
		case <-s.groupCtx.Done():
			s.Stop()
		}
	}()

	return s.group.Wait()
}

// Stop the servers in an orderly, graceful fashion
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
		if err != http.ErrServerClosed {
			s.logger.Fatal().Err(err).Msg("HTTP Server, stopping agent")
			return errors.Wrap(err, "HTTP server")
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
		if err != http.ErrServerClosed {
			s.logger.Fatal().Err(err).Msg("SSL Server, stopping agent")
			return errors.Wrap(err, "SSL server")
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
		if err != http.ErrServerClosed {
			s.logger.Fatal().Err(err).Str("socket", svr.address.String()).Msg("Socket Server, stopping agent")
			return errors.Wrap(err, "socket server")
		}
	}
	return nil
}
