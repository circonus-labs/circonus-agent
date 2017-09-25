// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package agent

import (
	"context"
	"os"
	"os/signal"

	"github.com/circonus-labs/circonus-agent/internal/plugins"
	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/circonus-labs/circonus-agent/internal/reverse"
	"github.com/circonus-labs/circonus-agent/internal/server"
	"github.com/circonus-labs/circonus-agent/internal/statsd"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// Agent holds the main circonus-agent process
type Agent struct {
	signalCh     chan os.Signal
	shutdown     func()
	shutdownCtx  context.Context
	errCh        chan error
	plugins      *plugins.Plugins
	listenServer *server.Server
	reverseConn  *reverse.Connection
	statsdServer *statsd.Server
}

// New returns a new agent instance
func New() (*Agent, error) {
	a := Agent{
		errCh:    make(chan error),
		signalCh: make(chan os.Signal, 10),
	}

	// Handle shutdown via a.shutdownCtx
	signalNotifySetup(a.signalCh)

	a.shutdownCtx, a.shutdown = context.WithCancel(context.Background())

	var err error

	a.plugins = plugins.New(a.shutdownCtx)
	err = a.plugins.Scan()
	if err != nil {
		return nil, err
	}

	a.statsdServer, err = statsd.New(a.shutdownCtx)
	if err != nil {
		return nil, err
	}

	a.reverseConn, err = reverse.New(a.shutdownCtx)
	if err != nil {
		return nil, err
	}

	a.listenServer, err = server.New(a.shutdownCtx, a.plugins, a.statsdServer)
	if err != nil {
		return nil, err
	}

	return &a, nil
}

// Start the agent
func (a *Agent) Start() {

	go a.handleSignals()

	go func() {
		if err := a.statsdServer.Start(); err != nil {
			a.errCh <- errors.Wrap(err, "Starting StatsD listener")
		}
	}()

	go func() {
		if err := a.reverseConn.Start(); err != nil {
			a.errCh <- errors.Wrap(err, "Unable to start reverse connection")
		}
	}()

	go func() {
		if err := a.listenServer.Start(); err != nil {
			a.errCh <- errors.Wrap(err, "Starting server")
		}
	}()
}

// Stop cleans up and shuts down the Agent
func (a *Agent) Stop() {
	a.stopSignalHandler()
	a.plugins.Stop()
	a.statsdServer.Stop()
	a.reverseConn.Stop()
	a.listenServer.Stop()
	a.shutdown()

	log.Debug().
		Int("pid", os.Getpid()).
		Str("name", release.NAME).
		Str("ver", release.VERSION).Msg("Stopped")

	os.Exit(0)
}

// Wait blocks until shutdown
func (a *Agent) Wait() error {
	log.Debug().
		Int("pid", os.Getpid()).
		Str("name", release.NAME).
		Str("ver", release.VERSION).Msg("Starting wait")
	select {
	case <-a.shutdownCtx.Done():
	case err := <-a.errCh:
		log.Error().Err(err).Msg("Shutting down agent due to errors")
		a.Stop()
		return err
	}

	return nil
}

// stopSignalHandler disables the signal handler
func (a *Agent) stopSignalHandler() {
	signal.Stop(a.signalCh)
}
