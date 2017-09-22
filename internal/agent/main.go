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
	"runtime"

	"github.com/alecthomas/units"
	"github.com/circonus-labs/circonus-agent/internal/plugins"
	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/circonus-labs/circonus-agent/internal/reverse"
	"github.com/circonus-labs/circonus-agent/internal/server"
	"github.com/circonus-labs/circonus-agent/internal/statsd"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/sys/unix"
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
		plugins:  plugins.New(),
	}

	// Handle shutdown via a.shutdownCtx
	signal.Notify(a.signalCh, os.Interrupt, unix.SIGTERM, unix.SIGHUP, unix.SIGPIPE, unix.SIGINFO)

	a.shutdownCtx, a.shutdown = context.WithCancel(context.Background())

	if err := a.plugins.Scan(); err != nil {
		return nil, err
	}

	{
		var err error
		a.statsdServer, err = statsd.New()
		if err != nil {
			return nil, err
		}
	}

	a.reverseConn = reverse.New()

	a.listenServer = server.New(a.plugins, a.statsdServer)

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
	log.Debug().Msg("Stopping signal handler")
	a.stopSignalHandler()

	log.Debug().Msg("Stopping plugins")
	a.plugins.Stop()

	log.Debug().Msg("Stopping statsd")
	a.statsdServer.Stop()

	log.Debug().Msg("Stopping reverse")
	a.reverseConn.Stop()

	log.Debug().Msg("Stopping listener")
	a.listenServer.Stop()

	log.Debug().Msg("Stopped " + release.NAME + " agent")
	os.Exit(0)
}

// Wait blocks until shutdown
func (a *Agent) Wait() error {
	log.Debug().Msg("Starting wait")
	select {
	case <-a.shutdownCtx.Done():
	case err := <-a.errCh:
		a.Stop()
		return err
	}

	return nil
}

// handleSignals runs the signal handler thread
func (a *Agent) handleSignals() {
	const stacktraceBufSize = 1 * units.MiB

	// pre-allocate a buffer
	buf := make([]byte, stacktraceBufSize)

	for {
		select {
		case <-a.shutdownCtx.Done():
			log.Debug().Msg("Shutting down")
			return
		case sig := <-a.signalCh:
			log.Info().Str("signal", sig.String()).Msg("Received signal")
			switch sig {
			case os.Interrupt, unix.SIGTERM:
				a.shutdown()
			case unix.SIGPIPE, unix.SIGHUP:
				// Noop
			case unix.SIGINFO:
				stacklen := runtime.Stack(buf, true)
				fmt.Printf("=== received SIGINFO ===\n*** goroutine dump...\n%s\n*** end\n", buf[:stacklen])
			default:
				panic(fmt.Sprintf("unsupported signal: %v", sig))
			}
		}
	}
}

// stopSignalHandler disables the signal handler
func (a *Agent) stopSignalHandler() {
	signal.Stop(a.signalCh)
}
