// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"context"
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
func New(ctx context.Context, p *plugins.Plugins, ss *statsd.Server) (*Server, error) {
	s := Server{
		ctx:       ctx,
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

	return &s, nil
}

// Start main listening server(s)
func (s *Server) Start() error {
	if s.svrHTTP == nil && s.svrHTTPS == nil {
		return errors.New("No servers defined")
	}
	// Manual waitgroup for the situation where both servers are started;
	// one fails and the other doesn't - wg.Wait() would block.
	// The desired behavior is for an error in *either* to abort the process (somewhat cleanly).
	// there is probably a more idiomatic way to handle this...
	numDone := 0
	expected := 0
	ec := make(chan error)
	done := make(chan int)

	if s.svrHTTP == nil {
		s.logger.Debug().Msg("No listen configured, skipping server")
	} else {
		expected++
		go func() {
			defer s.svrHTTP.Close()
			s.logger.Info().Str("listen", s.svrHTTP.Addr).Msg("Starting")
			if err := s.svrHTTP.ListenAndServe(); err != nil {
				if err != http.ErrServerClosed {
					ec <- err
				}
			}
			done <- 1
		}()
	}

	if s.svrHTTPS == nil {
		s.logger.Debug().Msg("No SSL listen configured, skipping server")
	} else {
		expected++
		go func() {
			defer s.svrHTTPS.Close()
			certFile := viper.GetString(config.KeySSLCertFile)
			keyFile := viper.GetString(config.KeySSLKeyFile)
			s.logger.Info().Str("listen", s.svrHTTPS.Addr).Msg("SSL starting")
			if err := s.svrHTTPS.ListenAndServeTLS(certFile, keyFile); err != nil {
				if err != http.ErrServerClosed {
					ec <- errors.Wrap(err, "SSL server")
				}
			}
			done <- 1
		}()
	}

	for numDone < expected {
		select {
		case <-s.ctx.Done():
			return nil
		case err := <-ec:
			return err
		case <-done:
			numDone++
			s.logger.Debug().Int("done", numDone).Msg("completed")
		}
	}

	return nil
}

// Stop the servers
func (s *Server) Stop() {
	if s.svrHTTP != nil {
		s.logger.Debug().Msg("Stopping HTTP server")
		err := s.svrHTTP.Close()
		if err != nil {
			s.logger.Warn().Err(err).Msg("Closing HTTP server")
		}
	}

	if s.svrHTTPS != nil {
		s.logger.Debug().Msg("Stopping HTTPS server")
		err := s.svrHTTPS.Close()
		if err != nil {
			s.logger.Warn().Err(err).Msg("Closing HTTPS server")
		}
	}
}
