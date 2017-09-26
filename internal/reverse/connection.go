// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/pkg/errors"
)

// connect to broker via w/tls and send initial introduction to start reverse
// NOTE: all reverse connections require tls
func (c *Connection) connect() error {
	c.logger.Info().Str("host", c.reverseURL.Host).Msg("Connecting")

	c.conn = nil

	c.connAttempts++
	dialer := &net.Dialer{Timeout: c.dialerTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", c.reverseURL.Host, c.tlsConfig)
	if err != nil {
		return errors.Wrapf(err, "connecting to %s", c.reverseURL.Host)
	}

	conn.SetDeadline(time.Now().Add(c.commTimeout))
	introReq := "REVERSE " + c.reverseURL.Path
	if c.reverseURL.Fragment != "" {
		introReq += "#" + c.reverseURL.Fragment // reverse secret is placed here when reverse url is parsed
	}
	c.logger.Debug().Msg(fmt.Sprintf("sending intro '%s'", introReq))
	if _, err := fmt.Fprintf(conn, "%s HTTP/1.1\r\n\r\n", introReq); err != nil {
		if err != nil {
			return errors.Wrapf(err, "unable to write intro to %s", c.reverseURL.Host)
		}
	}

	c.conn = conn

	return nil
}

// processCommands coming from broker
func (c *Connection) processCommands(ctx context.Context) error {
	defer c.conn.Close()

	for {
		select {
		case <-ctx.Done():
			return nil
		default: // fall out of select
		}

		c.conn.SetDeadline(time.Now().Add(c.commTimeout))
		nc, err := c.getCommandFromBroker(c.conn)
		if err != nil {
			return errors.Wrap(err, "getting command from broker")
		}
		if nc == nil {
			continue
		}

		if nc.command != noitCmdConnect {
			c.logger.Debug().Str("cmd", nc.command).Msg("ignoring command")
			continue
		}

		if len(nc.request) == 0 {
			c.logger.Debug().
				Str("cmd", nc.command).
				Str("req", string(nc.request)).
				Msg("ignoring zero length request")
			continue
		}

		if c.connAttempts > 1 {
			// successfully connected, sent, and received data
			// a broker can, in certain circumstances, allow a connection, accept
			// the initial introduction, and then summarily disconnect (e.g. multiple
			// agents attempting reverse connections for the same check.)
			c.resetConnectionAttempts()
		}

		// send the request from the broker to the local agent
		data, err := c.fetchMetricData(&nc.request)
		if err != nil {
			c.logger.Warn().Err(err).Msg("fetching metric data")
		}

		c.conn.SetDeadline(time.Now().Add(c.commTimeout))

		// send the metrics received from the local agent back to the broker
		if err := c.sendMetricData(c.conn, nc.channelID, data); err != nil {
			return errors.Wrap(err, "sending metric data") // restart the connection
		}
	}
}

// setNextDelay for failed connection attempts
func (c *Connection) setNextDelay() {
	if c.delay == c.maxDelay {
		return
	}

	if c.delay < c.maxDelay {
		drift := rand.Intn(maxDelayStep-minDelayStep) + minDelayStep
		c.delay += time.Duration(drift) * time.Second
	}

	if c.delay > c.maxDelay {
		c.delay = c.maxDelay
	}

	return
}

// resetConnectionAttempts on successful send/receive
func (c *Connection) resetConnectionAttempts() {
	if c.connAttempts > 0 {
		c.delay = 1 * time.Second
		c.connAttempts = 0
	}
}
