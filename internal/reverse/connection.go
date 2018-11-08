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

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// startReverse manages the actual reverse connection to the Circonus broker
func (c *Connection) startReverse() error {
	for {
		conn, cerr := c.connect()
		if cerr != nil {
			if cerr.fatal {
				c.logger.Fatal().Err(cerr.err).Msg("unable to establish reverse connection to broker")
			}
			c.logger.Warn().Err(cerr.err).Msg("retrying")
			continue
		}

		if c.shutdown() {
			return nil
		}

		done := make(chan interface{})
		commandReader := c.newCommandReader(done, conn)
		commandProcessor := c.newCommandProcessor(done, commandReader)
		for result := range commandProcessor {
			if c.shutdown() {
				close(done)
				conn.Close()
				return nil
			}
			if result.ignore {
				// c.logger.Debug().Err(result.err).Int("timeouts", c.commTimeouts).Msg("ignored")
				continue
			}
			if result.err != nil {
				if result.reset {
					c.logger.Warn().Err(result.err).Int("timeouts", c.commTimeouts).Msg("resetting connection")
					close(done)
					break
				} else if result.fatal {
					c.logger.Error().Err(result.err).Interface("result", result).Msg("fatal error, exiting")
					conn.Close()
					close(done)
					return result.err
				} else {
					c.logger.Error().Err(result.err).Interface("result", result).Msg("unhandled error state...")
					continue
				}
			}

			// send metrics to broker
			if err := c.sendMetricData(conn, result.channelID, result.metrics); err != nil {
				c.logger.Warn().Err(err).Msg("sending metric data, resetting connection")
				close(done)
				break
			}

			c.resetConnectionAttempts()
		}

		conn.Close()
		if c.shutdown() {
			return nil
		}

	}
}

// connect to broker w/tls and send initial introduction
// NOTE: all reverse connections require tls
func (c *Connection) connect() (*tls.Conn, *connError) {
	c.Lock()
	if c.connAttempts > 0 {
		if c.maxConnRetry != -1 && c.connAttempts >= c.maxConnRetry {
			c.Unlock()
			return nil, &connError{fatal: true, err: errors.Errorf("max broker connection attempts reached (%d of %d)", c.connAttempts, c.maxConnRetry)}
		}

		c.logger.Info().
			Str("delay", c.delay.String()).
			Int("attempt", c.connAttempts).
			Msg("connect retry")

		time.Sleep(c.delay)
		c.delay = c.getNextDelay(c.delay)

		// Under normal circumstances the configuration for reverse is
		// non-volatile. There are, however, some situations where the
		// configuration must be rebuilt. (e.g. ip of broker changed,
		// check changed to use a different broker, broker certificate
		// changes, etc.) The majority of configuration based errors are
		// fatal, no attempt is made to resolve.
		if c.connAttempts%c.configRetryLimit == 0 {
			c.logger.Info().Int("attempts", c.connAttempts).Msg("reconfig triggered")
			if err := c.check.RefreshCheckConfig(); err != nil {
				c.Unlock()
				return nil, &connError{fatal: false, err: errors.Wrap(err, "refreshing check configuration")}
			}
			c.logger.Debug().Msg("setting reverse config")
			rc, err := c.check.GetReverseConfig()
			if err != nil {
				c.Unlock()
				return nil, &connError{fatal: true, err: errors.Wrap(err, "reconfiguring reverse connection")}
			}
			if rc == nil {
				c.Unlock()
				return nil, &connError{fatal: true, err: errors.New("invalid reverse configuration (nil)")}
			}
			c.revConfig = *rc
			c.logger = log.With().Str("pkg", "reverse").Str("cid", viper.GetString(config.KeyCheckBundleID)).Logger()
			c.logger.Info().
				Str("rev_host", c.revConfig.ReverseURL.Hostname()).
				Str("rev_port", c.revConfig.ReverseURL.Port()).
				Str("rev_path", c.revConfig.ReverseURL.Path).
				Str("agent", c.agentAddress).
				Msg("reverse configuration")
		}
	}
	c.Unlock()

	revHost := c.revConfig.ReverseURL.Host
	c.logger.Debug().Str("host", revHost).Msg("connecting")
	c.Lock()
	c.connAttempts++
	c.Unlock()
	dialer := &net.Dialer{Timeout: c.dialerTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", c.revConfig.BrokerAddr.String(), c.revConfig.TLSConfig)
	if err != nil {
		return nil, &connError{fatal: false, err: errors.Wrapf(err, "connecting to %s", revHost)}
	}
	c.logger.Info().Str("host", revHost).Msg("connected")

	conn.SetDeadline(time.Now().Add(c.commTimeout))
	introReq := "REVERSE " + c.revConfig.ReverseURL.Path
	if c.revConfig.ReverseURL.Fragment != "" {
		introReq += "#" + c.revConfig.ReverseURL.Fragment // reverse secret is placed here when reverse url is parsed
	}
	c.logger.Debug().Msg(fmt.Sprintf("sending intro '%s'", introReq))
	if _, err := fmt.Fprintf(conn, "%s HTTP/1.1\r\n\r\n", introReq); err != nil {
		if err != nil {
			c.logger.Error().Err(err).Msg("sending intro")
			return nil, &connError{fatal: false, err: errors.Wrapf(err, "unable to write intro to %s", revHost)}
		}
	}

	c.Lock()
	// reset timeouts after successful (re)connection
	c.commTimeouts = 0
	c.Unlock()

	return conn, nil
}

// getNextDelay for failed connection attempts
func (c *Connection) getNextDelay(currDelay time.Duration) time.Duration {
	if currDelay == c.maxDelay {
		return currDelay
	}

	delay := currDelay

	if delay < c.maxDelay {
		drift := rand.Intn(c.maxDelayStep-c.minDelayStep) + c.minDelayStep
		delay += time.Duration(drift) * time.Second
	}

	if delay > c.maxDelay {
		delay = c.maxDelay
	}

	return delay
}

// resetConnectionAttempts on successful send/receive
func (c *Connection) resetConnectionAttempts() {
	c.Lock()
	if c.connAttempts > 0 {
		c.delay = 1 * time.Second
		c.connAttempts = 0
	}
	c.Unlock()
}

// Error returns string representation of a connError
func (e *connError) Error() string {
	return e.err.Error()
}
