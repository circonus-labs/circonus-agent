// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package connection

import (
	"context"
	"fmt"
)

func (c *Connection) newCommandProcessor(ctx context.Context, cmds <-chan command) <-chan command {
	commandResults := make(chan command)
	go func() {
		defer close(commandResults)
		for cmd := range cmds {
			cmdResult := c.processCommand(cmd)
			select {
			case <-ctx.Done():
				c.logger.Debug().Msg("stopping cmd processor (ctx)")
				return
			case commandResults <- cmdResult:
			}
		}
	}()
	return commandResults
}

func (c *Connection) processCommand(cmd command) command {
	if cmd.err != nil {
		return cmd
	}

	if cmd.name == CommandReset {
		cmd.err = fmt.Errorf("received %s command from broker", cmd.name) //nolint:goerr113
		cmd.ignore = false
		cmd.reset = true
		return cmd
	}

	if cmd.name != CommandConnect {
		cmd.ignore = true
		cmd.err = fmt.Errorf("unused/empty command (%s)", cmd.name) //nolint:goerr113
		return cmd
	}

	if len(cmd.request) == 0 {
		cmd.err = fmt.Errorf("invalid connect command, 0 length request") //nolint:goerr113
		return cmd
	}

	metrics, err := c.fetchMetricData(&cmd.request, cmd.channelID)
	if err != nil {
		cmd.err = fmt.Errorf("fetching metrics: %w", err)
		return cmd
	}

	cmd.metrics = metrics
	return cmd
}
