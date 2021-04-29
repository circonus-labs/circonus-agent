// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package agent

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/circonus-labs/circonus-agent/internal/builtins"
	"github.com/circonus-labs/circonus-agent/internal/check"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-agent/internal/multiagent"
	"github.com/circonus-labs/circonus-agent/internal/plugins"
	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/circonus-labs/circonus-agent/internal/reverse"
	"github.com/circonus-labs/circonus-agent/internal/server"
	"github.com/circonus-labs/circonus-agent/internal/statsd"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

// Agent holds the main circonus-agent process.
type Agent struct {
	group        *errgroup.Group
	groupCtx     context.Context
	groupCancel  context.CancelFunc
	builtins     *builtins.Builtins
	check        *check.Check
	listenServer *server.Server
	plugins      *plugins.Plugins
	reverseConn  *reverse.Reverse
	submitter    *multiagent.Submitter
	signalCh     chan os.Signal
	statsdServer *statsd.Server
	logger       zerolog.Logger
}

// New returns a new agent instance.
func New() (*Agent, error) {
	ctx, cancel := context.WithCancel(context.Background())
	g, gctx := errgroup.WithContext(ctx)

	var err error
	a := Agent{
		group:       g,
		groupCtx:    gctx,
		groupCancel: cancel,
		signalCh:    make(chan os.Signal, 10),
		logger:      log.With().Str("pkg", "agent").Logger(),
	}

	err = config.Validate()
	if err != nil {
		return nil, fmt.Errorf("config validate: %w", err)
	}

	a.check, err = check.New(nil)
	if err != nil {
		return nil, fmt.Errorf("init check: %w", err)
	}

	a.builtins, err = builtins.New(a.groupCtx)
	if err != nil {
		return nil, fmt.Errorf("init builtins: %w", err)
	}

	a.plugins, err = plugins.New(a.groupCtx, defaults.PluginPath)
	if err != nil {
		return nil, fmt.Errorf("init plugins: %w", err)
	}
	if err = a.plugins.Scan(a.builtins); err != nil {
		return nil, fmt.Errorf("scan plugins: %w", err)
	}

	a.statsdServer, err = statsd.New(a.groupCtx)
	if err != nil {
		return nil, fmt.Errorf("init statsd: %w", err)
	}

	a.listenServer, err = server.New(a.groupCtx, a.check, a.builtins, a.plugins, a.statsdServer)
	if err != nil {
		return nil, fmt.Errorf("init server: %w", err)
	}

	agentAddress, err := a.listenServer.GetReverseAgentAddress()
	if err != nil {
		return nil, fmt.Errorf("agent addr: %w", err)
	}

	if viper.GetBool(config.KeyReverse) {
		a.reverseConn, err = reverse.New(a.logger, a.check, agentAddress)
		if err != nil {
			return nil, fmt.Errorf("init reverse: %w", err)
		}
	}

	if viper.GetBool(config.KeyMultiAgent) {
		a.submitter, err = multiagent.New(a.logger, a.check, a.listenServer)
		if err != nil {
			return nil, fmt.Errorf("init multi-agent: %w", err)
		}
	}

	a.signalNotifySetup()

	return &a, nil
}

// Start the agent.
func (a *Agent) Start() error {
	a.group.Go(a.handleSignals)
	a.group.Go(a.statsdServer.Start)
	if viper.GetBool(config.KeyReverse) {
		a.group.Go(func() error {
			if err := a.reverseConn.Start(a.groupCtx); err != nil {
				return fmt.Errorf("start reverse: %w", err)
			}
			return nil
		})
	}
	if viper.GetBool(config.KeyMultiAgent) {
		a.group.Go(func() error {
			if err := a.submitter.Start(a.groupCtx); err != nil {
				return fmt.Errorf("start submitter: %w", err)
			}
			return nil
		})
	}
	a.group.Go(a.listenServer.Start)

	a.logger.Debug().
		Int("pid", os.Getpid()).
		Str("name", release.NAME).
		Str("ver", release.VERSION).Msg("Starting wait")

	if err := a.group.Wait(); err != nil {
		return fmt.Errorf("start agent: %w", err)
	}
	return nil
}

// Stop cleans up and shuts down the Agent.
func (a *Agent) Stop() {
	a.stopSignalHandler()
	a.groupCancel()

	a.logger.Debug().
		Int("pid", os.Getpid()).
		Str("name", release.NAME).
		Str("ver", release.VERSION).Msg("Stopped")
}

// stopSignalHandler disables the signal handler.
func (a *Agent) stopSignalHandler() {
	signal.Stop(a.signalCh)
	signal.Reset() // so a second ctrl-c will force immediate stop
}
