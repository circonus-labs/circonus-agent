// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package connection

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/pkg/errors"
)

func (c *Connection) newCommandReader(ctx context.Context, conn *tls.Conn) <-chan command {
	commandReader := make(chan command)
	go func() {
		defer close(commandReader)
		for {
			cmd := c.readCommand(conn)
			select {
			case <-ctx.Done():
				c.logger.Debug().Msg("stopping cmd reader (ctx)")
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
		// ignore first c.maxCommTimeout errors; workaround for conn.Read
		// being blocking and not interruptable with a context/channel
		// so that a request to stop will only block for a short period of time
		reset := true
		ignore := false
		if ne, ok := err.(*net.OpError); ok {
			if ne.Timeout() {
				c.Lock()
				c.commTimeouts++
				if c.commTimeouts <= MaxCommTimeouts {
					reset = false
					ignore = true
				}
				c.Unlock()
			}
		}
		return command{err: errors.Wrap(err, "reading command"), reset: reset, ignore: ignore}
	}

	c.Lock()
	c.commTimeouts = 0
	c.Unlock()

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

	if cmd.name == CommandConnect {
		// connect command requires a request
		cmd.start = time.Now()
		reqPkt, err := c.readFrameFromBroker(r)
		if err != nil {
			// ignore first c.maxCommTimeout errors; workaround for conn.Read
			// being blocking and not interruptable with a context/channel
			// so that a request to stop will only block for a short period of time
			reset := true
			ignore := false
			if ne, ok := err.(*net.OpError); ok {
				if ne.Timeout() {
					c.Lock()
					c.commTimeouts++
					if c.commTimeouts <= MaxCommTimeouts {
						reset = false
						ignore = true
					}
					c.Unlock()
				}
			}
			return command{err: errors.Wrap(err, "reading command payload"), reset: reset, ignore: ignore}
		}

		c.Lock()
		c.commTimeouts = 0
		c.Unlock()

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
