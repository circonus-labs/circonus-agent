// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	crand "crypto/rand"
	"math"
	"math/big"
	"math/rand"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/check"
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
func New(check *check.Check, agentAddress string) (*Connection, error) {
	const (
		// NOTE: TBD, make some of these user-configurable
		commTimeoutSeconds    = 10 // seconds, when communicating with noit
		dialerTimeoutSeconds  = 15 // seconds, establishing connection
		metricTimeoutSeconds  = 50 // seconds, when communicating with agent
		maxDelaySeconds       = 60 // maximum amount of delay between attempts
		maxRequests           = -1 // max requests from broker before resetting connection, -1 = unlimited
		brokerMaxRetries      = 5
		brokerMaxResponseTime = 500 * time.Millisecond
		brokerActiveStatus    = "active"
	)

	if check == nil {
		return nil, errors.New("invalid check value (empty)")
	}
	if agentAddress == "" {
		return nil, errors.New("invalid agent address (empty)")
	}
	c := Connection{
		agentAddress:     agentAddress,
		check:            check,
		commTimeout:      commTimeoutSeconds * time.Second,
		connAttempts:     0,
		delay:            1 * time.Second,
		dialerTimeout:    dialerTimeoutSeconds * time.Second,
		enabled:          viper.GetBool(config.KeyReverse),
		logger:           log.With().Str("pkg", "reverse").Logger(),
		maxDelay:         maxDelaySeconds * time.Second,
		metricTimeout:    metricTimeoutSeconds * time.Second,
		cmdConnect:       "CONNECT",
		cmdReset:         "RESET",
		maxPayloadLen:    65529,                                       // max unsigned short - 6 (for header)
		maxCommTimeouts:  5,                                           // multiply by commTimeout, ensure >(broker polling interval) otherwise conn reset loop
		minDelayStep:     1,                                           // minimum seconds to add on retry
		maxDelayStep:     20,                                          // maximum seconds to add on retry
		maxConnRetry:     viper.GetInt(config.KeyReverseMaxConnRetry), // max times to retry a persistently failing connection
		configRetryLimit: 5,                                           // if failed attempts > threshold, force reconfig
		maxRequests:      maxRequests,                                 // max requests from broker before reset
	}

	if c.enabled {
		c.logger.Info().Str("agent_address", c.agentAddress).Msg("reverse")
		rc, err := c.check.GetReverseConfig()
		if err != nil {
			return nil, errors.Wrap(err, "setting reverse config")
		}
		if rc == nil {
			return nil, errors.New("invalid reverse configuration (nil)")
		}
		c.revConfig = *rc
	}

	c.logger = log.With().Str("pkg", "reverse").Str("cid", viper.GetString(config.KeyCheckBundleID)).Logger()

	return &c, nil
}

// Start reverse connection to the broker
func (c *Connection) Start() error {
	if !c.enabled {
		c.logger.Info().Msg("Reverse disabled, not starting")
		return nil
	}

	c.logger.Info().
		Str("check_bundle", viper.GetString(config.KeyCheckBundleID)).
		Str("rev_host", c.revConfig.ReverseURL.Hostname()).
		Str("rev_port", c.revConfig.ReverseURL.Port()).
		Str("rev_path", c.revConfig.ReverseURL.Path).
		Str("agent", c.agentAddress).
		Msg("Reverse configuration")

	c.t.Go(c.startReverse)

	return c.t.Wait()
}

// Stop the reverse connection
func (c *Connection) Stop() {
	if !c.enabled {
		return
	}

	c.logger.Info().Msg("Stopping reverse connection")

	if c.t.Alive() {
		c.logger.Warn().Msg("Sent stop signal, may take a minute for timeout")
		c.t.Kill(nil)
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
