// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package agent

import (
	"context"
	"os"
	"os/signal"

	"github.com/circonus-labs/circonus-agent/internal/builtins"
	"github.com/circonus-labs/circonus-agent/internal/check"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-agent/internal/plugins"
	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/circonus-labs/circonus-agent/internal/reverse"
	"github.com/circonus-labs/circonus-agent/internal/server"
	"github.com/circonus-labs/circonus-agent/internal/statsd"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

// Agent holds the main circonus-agent process
type Agent struct {
	group        *errgroup.Group
	groupCtx     context.Context
	groupCancel  context.CancelFunc
	builtins     *builtins.Builtins
	check        *check.Check
	listenServer *server.Server
	plugins      *plugins.Plugins
	reverseConn  *reverse.Connection
	signalCh     chan os.Signal
	statsdServer *statsd.Server
}

// New returns a new agent instance
func New() (*Agent, error) {
	ctx, cancel := context.WithCancel(context.Background())
	g, gctx := errgroup.WithContext(ctx)

	var err error
	a := Agent{
		group:       g,
		groupCtx:    gctx,
		groupCancel: cancel,
		signalCh:    make(chan os.Signal, 10),
	}

	err = config.Validate()
	if err != nil {
		return nil, err
	}

	a.check, err = check.New(nil)
	if err != nil {
		return nil, err
	}

	a.builtins, err = builtins.New()
	if err != nil {
		return nil, err
	}

	a.plugins, err = plugins.New(a.groupCtx, defaults.PluginPath)
	if err != nil {
		return nil, err
	}
	if err = a.plugins.Scan(a.builtins); err != nil {
		return nil, err
	}

	a.statsdServer, err = statsd.New(a.groupCtx)
	if err != nil {
		return nil, err
	}

	a.listenServer, err = server.New(a.groupCtx, a.check, a.builtins, a.plugins, a.statsdServer)
	if err != nil {
		return nil, err
	}

	agentAddress, err := a.listenServer.GetReverseAgentAddress()
	if err != nil {
		return nil, err
	}
	a.reverseConn, err = reverse.New(a.groupCtx, a.check, agentAddress)
	if err != nil {
		return nil, err
	}

	a.signalNotifySetup()

	return &a, nil
}

// Start the agent
func (a *Agent) Start() error {
	a.group.Go(a.handleSignals)
	a.group.Go(a.statsdServer.Start)
	a.group.Go(a.reverseConn.Start)
	a.group.Go(a.listenServer.Start)

	log.Debug().
		Int("pid", os.Getpid()).
		Str("name", release.NAME).
		Str("ver", release.VERSION).Msg("Starting wait")

	return a.group.Wait()
}

// Stop cleans up and shuts down the Agent
func (a *Agent) Stop() {
	a.stopSignalHandler()
	a.groupCancel()

	log.Debug().
		Int("pid", os.Getpid()).
		Str("name", release.NAME).
		Str("ver", release.VERSION).Msg("Stopped")
}

// stopSignalHandler disables the signal handler
func (a *Agent) stopSignalHandler() {
	signal.Stop(a.signalCh)
	signal.Reset() // so a second ctrl-c will force immediate stop
}
