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
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/plugins"
	"github.com/circonus-labs/circonus-agent/internal/statsd"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"xi2.org/x/httpgzip"
)

// New creates a new instance of the listening servers
func New(p *plugins.Plugins, ss *statsd.Server) (*Server, error) {
	s := Server{
		logger:    log.With().Str("pkg", "server").Logger(),
		plugins:   p,
		statsdSvr: ss,
	}

	gzipHandler := httpgzip.NewHandler(http.HandlerFunc(s.router), []string{"application/json"})

	if addr := viper.GetString(config.KeyListen); addr != "" {
		s.svrHTTP = &http.Server{Addr: addr, Handler: gzipHandler}
		s.svrHTTP.SetKeepAlivesEnabled(false)
	}

	if addr := viper.GetString(config.KeySSLListen); addr != "" {
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
			certFile: certFile,
			keyFile:  keyFile,
			server:   &http.Server{Addr: addr, Handler: gzipHandler},
		}

		svr.server.SetKeepAlivesEnabled(false)
		s.svrHTTPS = &svr
	}

	socketList := viper.GetStringSlice(config.KeyListenSocket)
	if len(socketList) > 0 {
		for idx, addr := range socketList {
			ua, err := net.ResolveUnixAddr("unix", addr)
			if err != nil {
				s.logger.Error().Err(err).Int("id", idx).Str("addr", addr).Msg("resolving address")
				return nil, err
			}

			ul, err := net.ListenUnix(ua.Network(), ua)
			if err != nil {
				s.logger.Error().Err(err).Int("id", idx).Str("addr", addr).Msg("creating socket")
				return nil, err
			}

			s.svrSockets = append(s.svrSockets, socketServer{
				address:  ua,
				listener: ul,
				server:   &http.Server{Handler: http.HandlerFunc(s.socketHandler)},
			})
		}
	}

	if s.svrHTTP == nil && s.svrHTTPS == nil && len(s.svrSockets) == 0 {
		return nil, errors.New("No servers defined")
	}

	return &s, nil
}

// Start main listening server(s)
func (s *Server) Start() error {
	if s.svrHTTP == nil && s.svrHTTPS == nil && len(s.svrSockets) > 0 {
		return errors.New("No servers defined")
	}

	s.t.Go(s.startHTTP)
	s.t.Go(s.startHTTPS)
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
			if s.svrHTTP != nil {
				s.svrHTTP.Close()
			}
			if s.svrHTTPS != nil && s.svrHTTPS.server != nil {
				s.svrHTTPS.server.Close()
			}
			if len(s.svrSockets) == 0 {
				return // no sockets to close
			}
			for _, svr := range s.svrSockets {
				if svr.listener != nil {
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

	if s.svrHTTP != nil {
		s.logger.Info().Msg("Stopping HTTP server")
		err := s.svrHTTP.Shutdown(ctx)
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

func (s *Server) startHTTP() error {
	if s.svrHTTP == nil {
		s.logger.Debug().Msg("No listen configured, skipping server")
		return nil
	}
	s.logger.Info().Str("listen", s.svrHTTP.Addr).Msg("Starting")
	if err := s.svrHTTP.ListenAndServe(); err != nil {
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

func (s *Server) startSocket(svr socketServer) error {
	if svr.address == nil || svr.listener == nil || svr.server == nil {
		s.logger.Debug().Msg("No socket configured, skipping")
		return nil
	}

	s.logger.Info().Str("listen", svr.address.String()).Msg("Socket starting")
	if err := svr.server.Serve(svr.listener); err != nil {
		if err != http.ErrServerClosed {
			s.logger.Error().Err(err).Str("socket", svr.address.String()).Msg("Socket Server, stopping agent")
			return errors.Wrapf(err, "Socket (%s) server", svr.address.String())
		}
	}
	return nil
}
