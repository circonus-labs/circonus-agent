// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	"crypto/tls"
	"fmt"
	"io"

	"github.com/pkg/errors"
)

func (c *Connection) newCommandReader(done <-chan interface{}, conn *tls.Conn) <-chan command {
	commandReader := make(chan command)
	go func() {
		defer close(commandReader)
		for {
			cmd := c.readCommand(conn)
			select {
			case <-c.t.Dying():
				c.logger.Debug().Msg("reverse dying, cmd reader")
				return
			case <-done:
				c.logger.Debug().Msg("reverse 'done', cmd reader")
				return
			case commandReader <- cmd:
			}
		}
	}()
	return commandReader
}

func (c *Connection) readCommand(r io.Reader) command {
	cmdPkt, err := c.readFrameFromBroker(r)
	if err != nil {
		return command{err: errors.Wrap(err, "reading command"), reset: true}
	}

	if !cmdPkt.header.isCommand {
		c.logger.Warn().
			Str("cmd_header", fmt.Sprintf("%#v", cmdPkt.header)).
			Str("cmd_payload", string(cmdPkt.payload)).
			Msg("expected command")
		return command{err: errors.New("expected command")}
	}

	cmd := command{
		channelID: cmdPkt.header.channelID,
		name:      string(cmdPkt.payload),
	}

	if cmd.name == c.cmdConnect { // connect command requires a request
		reqPkt, err := c.readFrameFromBroker(r)
		if err != nil {
			cmd.err = errors.Wrap(err, "reading command payload")
			cmd.reset = true
			return cmd
		}

		if reqPkt.header.isCommand {
			c.logger.Warn().
				Str("cmd_header", fmt.Sprintf("%#v", cmdPkt.header)).
				Str("cmd_payload", string(cmdPkt.payload)).
				Str("req_header", fmt.Sprintf("%#v", reqPkt.header)).
				Str("req_payload", string(reqPkt.payload)).
				Msg("expected request")
			cmd.err = errors.New("expected request")
			return cmd
		}

		cmd.request = reqPkt.payload
	}

	return cmd
}

func (c *Connection) newCommandProcessor(done <-chan interface{}, cmds <-chan command) <-chan command {
	commandResults := make(chan command)
	go func() {
		defer close(commandResults)
		for cmd := range cmds {
			cmdResult := c.processCommand(cmd)
			select {
			case <-c.t.Dying():
				c.logger.Debug().Msg("reverse dying, cmd processor")
				return
			case <-done:
				c.logger.Debug().Msg("reverse 'done', cmd processor")
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

	if cmd.name == c.cmdReset {
		cmd.reset = true
		return cmd
	}

	if cmd.name != c.cmdConnect {
		cmd.ignore = true
		cmd.err = errors.Errorf("unused/empty command (%s)", cmd.name)
		return cmd
	}

	if len(cmd.request) == 0 {
		cmd.err = errors.New("invalid connect command, 0 length request")
		return cmd
	}

	metrics, err := c.fetchMetricData(&cmd.request)
	if err != nil {
		cmd.err = errors.Wrap(err, "fetching metrics")
		return cmd
	}

	cmd.metrics = metrics
	return cmd
}
