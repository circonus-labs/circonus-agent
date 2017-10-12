// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"net"
	"net/http"

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
		s.svrHTTPS = &http.Server{Addr: addr, Handler: gzipHandler}
		s.svrHTTPS.SetKeepAlivesEnabled(false)
	}

	if sp := viper.GetString(config.KeyListenSocketPath); sp != "" {
		l, err := net.Listen("unix", sp)
		if err != nil {
			return nil, errors.Wrap(err, "Creating socket")
		}
		s.svrSocket = &l
	}

	return &s, nil
}

// Start main listening server(s)
func (s *Server) Start() error {
	if s.svrHTTP == nil && s.svrHTTPS == nil {
		return errors.New("No servers defined")
	}

	s.t.Go(s.startHTTP)
	s.t.Go(s.startHTTPS)
	s.t.Go(s.startSocket)

	return s.t.Wait()
}

// Stop the servers
func (s *Server) Stop() {
	if s.svrHTTP != nil {
		s.logger.Info().Msg("Stopping HTTP server")
		err := s.svrHTTP.Close()
		if err != nil {
			s.logger.Warn().Err(err).Msg("Closing HTTP server")
		}
	}

	if s.svrHTTPS != nil {
		s.logger.Info().Msg("Stopping HTTPS server")
		err := s.svrHTTPS.Close()
		if err != nil {
			s.logger.Warn().Err(err).Msg("Closing HTTPS server")
		}
	}

	if s.svrSocket != nil {
		s.logger.Info().Msg("Stopping Socket server")
		err := (*s.svrSocket).Close()
		if err != nil {
			s.logger.Warn().Err(err).Msg("Closing Socket server")
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
	certFile := viper.GetString(config.KeySSLCertFile)
	keyFile := viper.GetString(config.KeySSLKeyFile)
	s.logger.Info().Str("listen", s.svrHTTPS.Addr).Msg("SSL starting")
	if err := s.svrHTTPS.ListenAndServeTLS(certFile, keyFile); err != nil {
		if err != http.ErrServerClosed {
			return errors.Wrap(err, "HTTPS server")
		}
	}
	return nil
}

func (s *Server) startSocket() error {
	if s.svrSocket == nil {
		s.logger.Debug().Msg("No Socket path configured, skipping server")
		return nil
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.logger.Debug().Str("method", r.Method).Interface("req_url", r.URL).Msg("got socket reqeust")

		if !writePathRx.MatchString(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		if r.Method != "PUT" && r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		s.write(w, r)
	})

	s.logger.Info().Str("listen", (*s.svrSocket).Addr().String()).Msg("Socket starting")
	if err := http.Serve(*s.svrSocket, handler); err != nil {
		if err != http.ErrServerClosed {
			return errors.Wrap(err, "Socket server")
		}
	}
	return nil
}
