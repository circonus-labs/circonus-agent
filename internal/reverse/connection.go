// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/pkg/errors"
)

// handler connects to the broker and reads commands sent by the broker
func (c *Connection) handler() error {
	defer close(c.cmdCh)
	for { // allow reconnecting
		select {
		case <-c.t.Dying():
			return nil
		default:
			cerr := c.connect()
			if cerr != nil {
				if cerr.fatal {
					return cerr
				}
				c.logger.Warn().Err(cerr).Int("attempt", c.connAttempts).Msg("connect failed")
			}
		}

		if c.conn == nil {
			continue
		}

		for {
			cmd, err := c.getCommandFromBroker(c.conn)
			select {
			case <-c.t.Dying():
				return nil
			default:
				// fall through
			}
			if err != nil {
				c.logger.Warn().Err(err).Msg("reading commands, resetting connection")
				break
			}
			c.cmdCh <- cmd
		}
	}
}

// processor handles commands from broker
func (c *Connection) processor() error {
	for {
		select {
		case <-c.t.Dying():
			return nil
		case nc := <-c.cmdCh:
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

			// Successfully connected, sent, and received data.
			// In certain circumstances, a broker will allow a connection, accept
			// the initial introduction, and then summarily disconnect (e.g. multiple
			// agents attempting reverse connections for the same check.)
			if c.connAttempts > 0 {
				c.resetConnectionAttempts()
			}

			// send the request from the broker to the local agent
			data, err := c.fetchMetricData(&nc.request)
			if err != nil {
				c.logger.Warn().Err(err).Msg("fetching metric data")
			}

			// send the metrics received from the local agent back to the broker
			if err := c.sendMetricData(c.conn, nc.channelID, data); err != nil {
				return errors.Wrap(err, "sending metric data") // restart the connection
			}
		}
	}
}

// connect to broker via w/tls and send initial introduction to start reverse
// NOTE: all reverse connections require tls
func (c *Connection) connect() *connError {
	if c.conn != nil {
		c.conn.Close()
	}

	c.Lock()
	c.conn = nil
	c.Unlock()

	if c.connAttempts > 0 {
		c.logger.Info().
			Str("delay", c.delay.String()).
			Int("attempt", c.connAttempts).
			Msg("connect retry")

		time.Sleep(c.delay)
		c.setNextDelay()

		// Under normal circumstances the configuration for reverse is
		// non-volatile. There are, however, some situations where the
		// configuration must be rebuilt. (e.g. ip of broker changed,
		// check changed to use a different broker, broker certificate
		// changes, etc.) The majority of configuration based errors are
		// fatal, no attempt is made to resolve.
		if c.connAttempts%configRetryLimit == 0 {
			c.logger.Info().Int("attempts", c.connAttempts).Msg("reconfig triggered")
			if err := c.setCheckConfig(); err != nil {
				return &connError{fatal: true, err: errors.Wrap(err, "reconfiguring reverse connection")}
			}
		}
	}

	select {
	case <-c.t.Dying():
		return nil
	default:
	}

	c.logger.Info().Str("host", c.reverseURL.Host).Msg("Connecting")
	c.Lock()
	c.connAttempts++
	c.Unlock()
	dialer := &net.Dialer{Timeout: c.dialerTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", c.reverseURL.Host, c.tlsConfig)
	if err != nil {
		if c.connAttempts >= maxConnRetry {
			return &connError{fatal: true, err: errors.Wrapf(err, "after %d failed attempts, last error", c.connAttempts)}
		}
		return &connError{fatal: false, err: errors.Wrapf(err, "connecting to %s", c.reverseURL.Host)}
	}

	conn.SetDeadline(time.Now().Add(c.commTimeout))
	introReq := "REVERSE " + c.reverseURL.Path
	if c.reverseURL.Fragment != "" {
		introReq += "#" + c.reverseURL.Fragment // reverse secret is placed here when reverse url is parsed
	}
	c.logger.Debug().Msg(fmt.Sprintf("sending intro '%s'", introReq))
	if _, err := fmt.Fprintf(conn, "%s HTTP/1.1\r\n\r\n", introReq); err != nil {
		if err != nil {
			return &connError{fatal: false, err: errors.Wrapf(err, "unable to write intro to %s", c.reverseURL.Host)}
		}
	}

	c.Lock()
	c.conn = conn
	c.Unlock()
	return nil
}

// setNextDelay for failed connection attempts
func (c *Connection) setNextDelay() {
	if c.delay == c.maxDelay {
		return
	}
	c.Lock()
	defer c.Unlock()

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
		c.Lock()
		c.delay = 1 * time.Second
		c.connAttempts = 0
		c.Unlock()
	}
}

// Error returns string representation of a connError
func (e *connError) Error() string {
	return e.err.Error()
}
