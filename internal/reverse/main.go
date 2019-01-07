// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	"context"
	crand "crypto/rand"
	"math"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/check"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

// Connection defines a reverse connection
type Connection struct {
	group            *errgroup.Group
	groupCtx         context.Context
	agentAddress     string
	check            *check.Check
	cmdConnect       string
	cmdReset         string
	commTimeout      time.Duration
	commTimeouts     int
	configRetryLimit int
	connAttempts     int
	delay            time.Duration
	dialerTimeout    time.Duration
	enabled          bool
	logger           zerolog.Logger
	maxCommTimeouts  int
	maxConnRetry     int
	maxDelay         time.Duration
	maxDelayStep     int
	maxPayloadLen    uint32
	maxRequests      int
	metricTimeout    time.Duration
	minDelayStep     int
	revConfig        check.ReverseConfig
	sync.Mutex
}

// noitHeader defines the header received from the noit/broker
type noitHeader struct {
	channelID  uint16
	isCommand  bool
	payloadLen uint32
}

// noitFrame defines the header + the payload (described by the header) received from the noit/broker
type noitFrame struct {
	header  *noitHeader
	payload []byte
}

// connError returned from connect(), adds flag indicating whether the error is
// a warning or fatal.
type connError struct {
	err   error
	fatal bool
}

// command contains details of the command received from the broker
type command struct {
	err       error
	ignore    bool
	fatal     bool
	reset     bool
	channelID uint16
	name      string
	request   []byte
	metrics   *[]byte
	start     time.Time
}

func init() {
	n, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		rand.Seed(time.Now().UTC().UnixNano())
		return
	}
	rand.Seed(n.Int64())
}

// New creates a new connection
func New(ctx context.Context, check *check.Check, agentAddress string) (*Connection, error) {
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
	g, gctx := errgroup.WithContext(ctx)
	c := Connection{
		group:            g,
		groupCtx:         gctx,
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
		c.logger.Info().Msg("disabled, not starting")
		return nil
	}

	c.logger.Info().
		Str("rev_host", c.revConfig.ReverseURL.Hostname()).
		Str("rev_port", c.revConfig.ReverseURL.Port()).
		Str("rev_path", c.revConfig.ReverseURL.Path).
		Str("agent", c.agentAddress).
		Msg("configuration")

	c.group.Go(c.startReverse)
	go func() {
		select {
		case <-c.groupCtx.Done():
			c.logger.Warn().Msg("sent stop signal, may take a minute for timeout")
		}
	}()

	return c.group.Wait()
}

// shutdown checks for context being done
func (c *Connection) shutdown() bool {
	select {
	case <-c.groupCtx.Done():
		return true
	default:
		return false
	}
}
