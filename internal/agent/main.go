// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package agent

import (
	"github.com/circonus-labs/circonus-agent/internal/plugins"
	"github.com/circonus-labs/circonus-agent/internal/reverse"
	"github.com/circonus-labs/circonus-agent/internal/server"
	"github.com/circonus-labs/circonus-agent/internal/statsd"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// Agent holds the main circonus-agent process
type Agent struct {
	errChan chan error
}

// New returns a new agent instance
func New() (*Agent, error) {
	a := Agent{}
	a.errChan = make(chan error)
	return &a, nil
}

// Start the agent
func (a *Agent) Start() {
	if err := plugins.Initialize(); err != nil {
		log.Fatal().Err(err).Msg("Initializing plugins")
		return
	}

	go func() {
		err := statsd.Start()
		if err != nil {
			a.errChan <- errors.Wrap(err, "Starting StatsD listener")
		}
	}()

	go func() {
		err := reverse.Start()
		if err != nil {
			a.errChan <- errors.Wrap(err, "Unable to start reverse connection")
		}
	}()

	go func() {
		err := server.Start()
		if err != nil {
			a.errChan <- errors.Wrap(err, "Starting server")
		}
	}()
}

// Stop the agent
func (a *Agent) Stop() {
	// noop
}

// Wait for agent components to exit
func (a *Agent) Wait() error {
	select {
	case err := <-a.errChan:
		return err
	}
}
