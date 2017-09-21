// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"net/http"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"xi2.org/x/httpgzip"
)

var logger zerolog.Logger

// Start main listening server(s)
func Start() error {
	logger = log.With().Str("pkg", "server").Logger()
	return runServers(serverHTTP(), serverHTTPS())
}

func runServers(svrHTTP *http.Server, svrHTTPS *http.Server) error {
	if svrHTTP == nil && svrHTTPS == nil {
		return errors.New("No servers defined")
	}

	// Manual waitgroup for the situation where both servers are started;
	// one fails and the other doesn't - wg.Wait() would block.
	// The desired behavior is for an error in *either* to abort the process.
	// there is probably a more idiomatic way to handle this...
	numDone := 0
	expected := 0
	ec := make(chan error)
	done := make(chan int)

	if svrHTTP == nil {
		logger.Debug().Msg("No listen configured, skipping server")
	} else {
		expected++
		go func() {
			defer svrHTTP.Close()
			logger.Info().Str("listen", svrHTTP.Addr).Msg("Starting")
			if err := svrHTTP.ListenAndServe(); err != nil {
				if err.Error() != "http: Server closed" {
					ec <- err
				}
			}
			done <- 1
		}()
	}

	if svrHTTPS == nil {
		logger.Debug().Msg("No SSL listen configured, skipping server")
	} else {
		expected++
		go func() {
			defer svrHTTPS.Close()
			certFile := viper.GetString(config.KeySSLCertFile)
			keyFile := viper.GetString(config.KeySSLKeyFile)
			logger.Info().Str("listen", svrHTTPS.Addr).Msg("SSL starting")
			if err := svrHTTPS.ListenAndServeTLS(certFile, keyFile); err != nil {
				ec <- errors.Wrap(err, "SSL server")
			}
			done <- 1
		}()
	}

	if expected > 0 {
		select {
		case err := <-ec:
			return err
		case <-done:
			numDone++
			logger.Debug().Int("done", numDone).Msg("completed")
			if numDone == expected {
				break
			}
		}
	}

	return nil
}

func serverHTTP() *http.Server {
	addr := viper.GetString(config.KeyListen)
	if addr == "" {
		return nil
	}

	gzipHandler := httpgzip.NewHandler(http.HandlerFunc(router), []string{"application/json"})
	server := &http.Server{Addr: addr, Handler: gzipHandler}
	server.SetKeepAlivesEnabled(false)
	return server
}

func serverHTTPS() *http.Server {
	addr := viper.GetString(config.KeySSLListen)
	if addr == "" {
		return nil
	}

	gzipHandler := httpgzip.NewHandler(http.HandlerFunc(router), []string{"application/json"})
	server := &http.Server{Addr: addr, Handler: gzipHandler}
	server.SetKeepAlivesEnabled(false)

	return server
}
