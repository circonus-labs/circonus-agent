// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	crand "crypto/rand"
	"errors"
	"math"
	"math/big"
	"math/rand"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
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
func New(agentAddress string) (*Connection, error) {
	if agentAddress == "" {
		return nil, errors.New("invalid agent address (empty)")
	}
	c := Connection{
		agentAddress:  agentAddress,
		checkCID:      viper.GetString(config.KeyReverseCID),
		cmdCh:         make(chan *noitCommand),
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
		c.logger.Info().Str("agent_address", c.agentAddress).Msg("reverse")
		err := c.setCheckConfig()
		if err != nil {
			return nil, err
		}
	}

	return &c, nil
}

// Start reverse connection to the broker
func (c *Connection) Start() error {
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

	c.t.Go(c.handler)
	c.t.Go(c.processor)

	return c.t.Wait()
}

// Stop the reverse connection
func (c *Connection) Stop() {
	if !c.enabled {
		return
	}

	c.logger.Info().Msg("Stopping reverse connection")

	if c.t.Alive() {
		c.t.Kill(nil)
	}

	if c.conn == nil {
		return
	}

	c.logger.Info().Msg("Closing reverse connection")
	err := c.conn.Close()
	if err != nil {
		c.logger.Warn().Err(err).Msg("Closing reverse connection")
	}
}

// shutdown checks whether tomb is dying
func (c *Connection) shutdown() bool {
	select {
	case <-c.t.Dying():
		return true
	default:
		return false
	}
}
