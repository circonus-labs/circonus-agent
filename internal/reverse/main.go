// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	"context"
	crand "crypto/rand"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func init() {
	n, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		rand.Seed(time.Now().UTC().UnixNano())
		return
	}
	rand.Seed(n.Int64())
}

// New creates a new connection
func New() (*Connection, error) {
	c := Connection{
		checkCID:      viper.GetString(config.KeyReverseCID),
		cmdCh:         make(chan noitCommand),
		commTimeout:   commTimeoutSeconds * time.Second,
		connAttempts:  0,
		delay:         1 * time.Second,
		dialerTimeout: dialerTimeoutSeconds * time.Second,
		enabled:       viper.GetBool(config.KeyReverse),
		logger:        log.With().Str("pkg", "reverse").Logger(),
		maxDelay:      maxDelaySeconds * time.Second,
		metricTimeout: metricTimeoutSeconds * time.Second,
	}

	if c.enabled {
		c.agentAddress = strings.Replace(viper.GetString(config.KeyListen), "0.0.0.0", "localhost", -1)
		err := c.setCheckConfig()
		if err != nil {
			return nil, err
		}
	}

	return &c, nil
}

// Start reverse connection to the broker
func (c *Connection) Start(ctx context.Context) error {
	if !c.enabled {
		c.logger.Info().Msg("Reverse disabled, not starting")
		return nil
	}

	c.logger.Info().
		Str("check_bundle", viper.GetString(config.KeyReverseCID)).
		Str("rev_host", c.reverseURL.Hostname()).
		Str("rev_port", c.reverseURL.Port()).
		Str("rev_path", c.reverseURL.Path).
		Str("agent", c.agentAddress).
		Msg("Reverse configuration")

	errCh := make(chan error, 10)

	go func() {
		var lastErr error
		for { // allow for restarts
			select {
			case <-ctx.Done():
				return
			default:
				err := c.connect()
				if err != nil {
					lastErr = err
					c.logger.Warn().Err(err).Int("attempt", c.connAttempts).Msg("failed")
				}
			}

			if c.conn != nil {
				select {
				case <-ctx.Done():
					return
				default:
					err := c.processCommands(ctx)
					if err != nil { // non-fatal, log and reconnect
						lastErr = err
						c.logger.Warn().Err(err).Int("attempt", c.connAttempts).Msg("failed")
					}
				}
			}

			select {
			case <-ctx.Done():
				return
			default:
				// retry n times on connection attempt failures
				if c.connAttempts >= maxConnRetry {
					errCh <- errors.Wrapf(lastErr, "after %d failed attempts, last error", c.connAttempts)
					return
				}

				c.logger.Info().
					Str("delay", c.delay.String()).
					Int("attempt", c.connAttempts).
					Msg("connect retry")

				time.Sleep(c.delay)
				c.setNextDelay()

				if c.connAttempts%configRetryLimit == 0 {
					// Under normal circumstances the configuration for reverse is
					// non-volatile. There are, however, some situations where the
					// configuration must be rebuilt. (e.g. ip of broker changed,
					// check changed to use a different broker, broker certificate
					// changes, etc.) The majority of configuration based errors are
					// fatal, no attempt is made to resolve.
					c.logger.Info().Int("attempts", c.connAttempts).Msg("reconfig triggered")
					if err := c.setCheckConfig(); err != nil {
						errCh <- errors.Wrap(err, "reconfiguring reverse connection")
						return
					}
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errCh:
			return err
		}
	}
}

// Stop the reverse connection
func (c *Connection) Stop() {
	if !c.enabled {
		return
	}
	c.logger.Debug().Msg("Stopping reverse connection")
	if c.conn == nil {
		return
	}
	err := c.conn.Close()
	if err != nil {
		c.logger.Warn().Err(err).Msg("Closing reverse connection")
		c.logger.Debug().Str("state", fmt.Sprintf("%+v", c.conn.ConnectionState())).Msg("conn state")
	}
}
