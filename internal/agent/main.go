// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package agent

import (
	"os"
	"os/signal"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/plugins"
	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/circonus-labs/circonus-agent/internal/reverse"
	"github.com/circonus-labs/circonus-agent/internal/server"
	"github.com/circonus-labs/circonus-agent/internal/statsd"
	"github.com/rs/zerolog/log"
)

// New returns a new agent instance
func New() (*Agent, error) {
	a := Agent{
		signalCh: make(chan os.Signal, 10),
	}

	var err error

	//
	// validate the configuration
	//
	err = config.Validate()
	if err != nil {
		return nil, err
	}

	a.plugins = plugins.New(a.t.Context(nil))
	err = a.plugins.Scan()
	if err != nil {
		return nil, err
	}

	a.statsdServer, err = statsd.New()
	if err != nil {
		return nil, err
	}

	a.reverseConn, err = reverse.New()
	if err != nil {
		return nil, err
	}

	a.listenServer, err = server.New(a.plugins, a.statsdServer)
	if err != nil {
		return nil, err
	}

	a.signalNotifySetup()

	return &a, nil
}

// Start the agent
func (a *Agent) Start() error {
	a.t.Go(a.handleSignals)
	a.t.Go(a.statsdServer.Start)
	a.t.Go(a.reverseConn.Start)
	a.t.Go(a.listenServer.Start)

	log.Debug().
		Int("pid", os.Getpid()).
		Str("name", release.NAME).
		Str("ver", release.VERSION).Msg("Starting wait")

	return a.t.Wait()
}

// Stop cleans up and shuts down the Agent
func (a *Agent) Stop() {
	a.stopSignalHandler()
	a.plugins.Stop()
	a.statsdServer.Stop()
	a.reverseConn.Stop()
	a.listenServer.Stop()

	log.Debug().
		Int("pid", os.Getpid()).
		Str("name", release.NAME).
		Str("ver", release.VERSION).Msg("Stopped")

	if a.t.Alive() {
		a.t.Kill(nil)
	}
}

// stopSignalHandler disables the signal handler
func (a *Agent) stopSignalHandler() {
	signal.Stop(a.signalCh)
	signal.Reset() // so a second ctrl-c will force a kill
}
