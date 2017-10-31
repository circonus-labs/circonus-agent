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
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-agent/internal/plugins"
	"github.com/circonus-labs/circonus-agent/internal/statsd"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"xi2.org/x/httpgzip"
)

// New creates a new instance of the listening servers
func New(b *builtins.Builtins, p *plugins.Plugins, ss *statsd.Server) (*Server, error) {
	s := Server{
		logger:    log.With().Str("pkg", "server").Logger(),
		builtins:  b,
		plugins:   p,
		statsdSvr: ss,
	}

	// HTTP listener (1-n)
	{
		serverList := viper.GetStringSlice(config.KeyListen)
		if len(serverList) == 0 {
			serverList = []string{defaults.Listen}
		}
		for idx, addr := range serverList {
			ta, err := parseListen(addr)
			if err != nil {
				s.logger.Error().Err(err).Int("id", idx).Str("addr", addr).Msg("resolving address")
				return nil, errors.Wrap(err, "HTTP Server")
			}

			svr := httpServer{
				address: ta,
				server: &http.Server{
					Addr:    ta.String(),
					Handler: httpgzip.NewHandler(http.HandlerFunc(s.router), []string{"application/json"}),
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
				Handler: httpgzip.NewHandler(http.HandlerFunc(s.router), []string{"application/json"}),
			},
		}

		svr.server.SetKeepAlivesEnabled(false)
		s.svrHTTPS = &svr
	}

	// Socket listener (1-n)
	{
		socketList := viper.GetStringSlice(config.KeyListenSocket)
		for idx, addr := range socketList {
			ua, err := net.ResolveUnixAddr("unix", addr)
			if err != nil {
				s.logger.Error().Err(err).Int("id", idx).Str("addr", addr).Msg("resolving address")
				return nil, errors.Wrap(err, "Socket server")
			}

			if _, err := os.Stat(ua.String()); err == nil || !os.IsNotExist(err) {
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

	// validation moved to New so, there will always be at least ONE http server
	// if len(s.svrHTTP) == 0 && s.svrHTTPS == nil && len(s.svrSockets) == 0 {
	// 	return nil, errors.New("No servers defined")
	// }

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

	s.t.Go(s.startHTTPS)
	for _, svrHTTP := range s.svrHTTP {
		s.t.Go(func() error {
			return s.startHTTP(svrHTTP)
		})
	}
	for _, svrSocket := range s.svrSockets {
		s.t.Go(func() error {
			return s.startSocket(svrSocket)
		})
	}

	// start a tomb dying listener so that if one server fails to start
	// all other servers will be stopped. since http.servers don't have
	// listen with context (yet) and will block waiting for a request
	// in order to receive <-s.t.Dying. this is more 'immediate'.
	go func() {
		select {
		case <-s.t.Dying():
			if s.t.Err() == nil { // don't fire if a normal s.Stop() was initiated
				return
			}
			if s.svrHTTPS != nil && s.svrHTTPS.server != nil {
				s.svrHTTPS.server.Close()
			}
			for _, svr := range s.svrHTTP {
				svr.server.Close()
			}
			for _, svr := range s.svrSockets {
				if svr.server != nil {
					svr.server.Close()
				} else if svr.listener != nil {
					svr.listener.Close()
				}
			}
		}
	}()

	return s.t.Wait()
}

// Stop the servers in an orderly, graceful fashion
func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, svrHTTP := range s.svrHTTP {
		s.logger.Info().Msg("Stopping HTTP server")
		err := svrHTTP.server.Shutdown(ctx)
		if err != nil {
			s.logger.Warn().Err(err).Msg("Closing HTTP server")
		}
	}

	if s.svrHTTPS != nil {
		s.logger.Info().Msg("Stopping HTTPS server")
		err := s.svrHTTPS.server.Shutdown(ctx)
		if err != nil {
			s.logger.Warn().Err(err).Msg("Closing HTTPS server")
		}
	}

	for _, svrSocket := range s.svrSockets {
		s.logger.Info().Str("server", svrSocket.address.Name).Msg("Stopping Socket server")
		err := svrSocket.server.Shutdown(ctx)
		if err != nil {
			s.logger.Warn().Err(err).Str("server", svrSocket.address.Name).Msg("Closing Socket server")
		}
	}

	if s.t.Alive() {
		s.t.Kill(nil)
	}
}

func (s *Server) startHTTP(svr *httpServer) error {
	if svr == nil {
		s.logger.Debug().Msg("No listen configured, skipping server")
		return nil
	}
	if svr.address == nil || svr.server == nil {
		s.logger.Debug().Msg("listen not configured, skipping server")
		return nil
	}

	s.logger.Info().Str("listen", svr.address.String()).Msg("Starting")
	if err := svr.server.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			s.logger.Error().Err(err).Msg("HTTP Server, stopping agent")
			return errors.Wrap(err, "HTTP server")
		}
	}
	return nil
}

func (s *Server) startHTTPS() error {
	if s.svrHTTPS == nil {
		s.logger.Debug().Msg("No SSL listen configured, skipping server")
		return nil
	}
	s.logger.Info().Str("listen", s.svrHTTPS.server.Addr).Msg("SSL starting")
	if err := s.svrHTTPS.server.ListenAndServeTLS(s.svrHTTPS.certFile, s.svrHTTPS.keyFile); err != nil {
		if err != http.ErrServerClosed {
			s.logger.Error().Err(err).Msg("SSL Server, stopping agent")
			return errors.Wrap(err, "SSL server")
		}
	}
	return nil
}

func (s *Server) startSocket(svr *socketServer) error {
	if svr == nil {
		s.logger.Debug().Msg("No socket configured, skipping")
		return nil
	}
	if svr.address == nil || svr.listener == nil || svr.server == nil {
		s.logger.Debug().Msg("socket not configured, skipping")
		return nil
	}

	s.logger.Info().Str("listen", svr.address.String()).Msg("Socket starting")
	if err := svr.server.Serve(svr.listener); err != nil {
		if err != http.ErrServerClosed {
			s.logger.Error().Err(err).Str("socket", svr.address.String()).Msg("Socket Server, stopping agent")
			return errors.Wrap(err, "socket server")
		}
	}
	return nil
}

// parseListen parses and fixes listen spec
func parseListen(spec string) (*net.TCPAddr, error) {
	// empty, default
	if spec == "" {
		spec = defaults.Listen
	}
	// only a port, prefix with colon
	if ok, _ := regexp.MatchString(`^[0-9]+$`, spec); ok {
		spec = ":" + spec
	}
	// ipv4 w/o port, add default
	if strings.Contains(spec, ".") && !strings.Contains(spec, ":") {
		spec += defaults.Listen
	}
	// ipv6 w/o port, add default
	if ok, _ := regexp.MatchString(`^\[[a-f0-9:]+\]$`, spec); ok {
		spec += defaults.Listen
	}

	host, port, err := net.SplitHostPort(spec)
	if err != nil {
		return nil, errors.Wrap(err, "parsing listen")
	}

	addr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(host, port))
	if err != nil {
		return nil, errors.Wrap(err, "resolving listen")
	}

	return addr, nil
}
