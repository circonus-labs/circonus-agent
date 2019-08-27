// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package connection

import (
	"context"
	crand "crypto/rand"
	"crypto/tls"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/check"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

type Connection struct {
	logger          zerolog.Logger
	State           string
	LastRequestTime *time.Time
	agentAddress    string
	commTimeouts    int
	connAttempts    int
	delay           time.Duration
	maxConnRetry    int
	revConfig       check.ReverseConfig
	sync.Mutex
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

// connError returned from connect(), adds flag indicating whether to retry
type connError struct {
	err   error
	retry bool
}

type OpError struct {
	Err          string
	Fatal        bool
	RefreshCheck bool
	OrigErr      error
}

func (e *OpError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err == "" {
		return e.OrigErr.Error()
	}
	return e.Err
}

const (
	StateConnActive = "CONN_ACTIVE" // connected, broker requesting metrics
	StateConnIdle   = "CONN_IDLE"   // connected, no requests
	StateNew        = "NEW"         // new, no attempt to connect yet
	StateError      = "ERROR"       // connection is erroring
	CommandConnect  = "CONNECT"     // Connect command, must be followed by a request payload
	CommandReset    = "RESET"       // Reset command, resets the connection

	// NOTE: TBD, make some of these user-configurable
	CommTimeoutSeconds   = 10    // seconds, when communicating with noit
	DialerTimeoutSeconds = 15    // seconds, establishing connection
	MetricTimeoutSeconds = 50    // seconds, when communicating with agent
	MaxDelaySeconds      = 60    // maximum amount of delay between attempts
	MaxRequests          = -1    // max requests from broker before resetting connection, -1 = unlimited
	MaxPayloadLen        = 65529 // max unsigned short - 6 (for header)
	MaxCommTimeouts      = 5     // multiply by commTimeout, ensure >(broker polling interval) otherwise conn reset loop
	MinDelayStep         = 1     // minimum seconds to add on retry
	MaxDelayStep         = 20    // maximum seconds to add on retry
	ConfigRetryLimit     = 5     // if failed attempts > limit, force check reconfig (see if broker configuration changed)
)

func New(parentLogger zerolog.Logger, agentAddress string, cfg *check.ReverseConfig) (*Connection, error) {
	if agentAddress == "" {
		return nil, errors.Errorf("invalid agent address (empty)")
	}
	if cfg == nil {
		return nil, errors.Errorf("invalid config (nil)")
	}

	if n, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64)); err != nil {
		rand.Seed(time.Now().UTC().UnixNano())
	} else {
		rand.Seed(n.Int64())
	}

	c := Connection{
		agentAddress: agentAddress,
		revConfig:    *cfg,
		State:        StateNew,
		logger:       parentLogger.With().Str("cn", cfg.CN).Logger(),
		connAttempts: 0,
		delay:        1 * time.Second,
		maxConnRetry: viper.GetInt(config.KeyReverseMaxConnRetry), // max times to retry a persistently failing connection
	}

	return &c, nil
}

// Start the reverse connection to the broker
func (c *Connection) Start(ctx context.Context) error {

	for {

		conn, cerr := c.connect(ctx)
		if cerr != nil {
			if cerr.retry {
				c.logger.Warn().Err(cerr.err).Msg("retrying")
				continue
			}
			c.logger.Error().Err(cerr.err).Msg("unable to establish reverse connection to broker")
			return &OpError{
				RefreshCheck: true,
				OrigErr:      cerr.err,
			}

		}

		select {
		case <-ctx.Done():
			return nil
		default:
		}

		cmdCtx, cmdCancel := context.WithCancel(ctx)
		defer cmdCancel()
		commandReader := c.newCommandReader(cmdCtx, conn)
		commandProcessor := c.newCommandProcessor(cmdCtx, commandReader)

		for result := range commandProcessor {

			select {
			case <-ctx.Done():
				conn.Close()
				return nil
			default:
			}

			if result.ignore {
				continue
			}

			if result.err != nil {
				switch {
				case result.reset:
					c.logger.Warn().Err(result.err).Int("timeouts", c.commTimeouts).Msg("resetting connection")
					cmdCancel()
					break
				case result.fatal:
					c.logger.Error().Err(result.err).Interface("result", result).Msg("fatal error, exiting")
					conn.Close()
					return &OpError{
						Fatal:   true,
						OrigErr: result.err,
					}
				default:
					c.logger.Error().Err(result.err).Interface("result", result).Msg("unhandled error state...")
					continue
				}
			}

			// send metrics to broker
			if err := c.sendMetricData(conn, result.channelID, result.metrics, result.start); err != nil {
				c.logger.Warn().Err(err).Msg("sending metric data, resetting connection")
				conn.Close()
				return &OpError{
					RefreshCheck: true,
					OrigErr:      err,
				}
			}

			c.Lock()
			c.State = StateConnActive
			reqTime := result.start
			c.LastRequestTime = &reqTime
			c.Unlock()

			c.logger.Debug().Uint16("channel_id", result.channelID).Str("duration", time.Since(result.start).String()).Msg("CONNECT command request processed")

			c.resetConnectionAttempts()
		}

		conn.Close()
		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
}

// connect to broker w/tls and send initial introduction
// NOTE: all reverse connections require tls
func (c *Connection) connect(ctx context.Context) (*tls.Conn, *connError) {
	c.Lock()
	if c.connAttempts > 0 {
		if c.maxConnRetry != -1 && c.connAttempts >= c.maxConnRetry {
			c.Unlock()
			return nil, &connError{retry: false, err: errors.Errorf("max broker connection attempts reached (%d of %d)", c.connAttempts, c.maxConnRetry)}
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
		// changes, cluster membership changes, etc.) The majority of
		// configuration based errors are fatal, no attempt is made to
		// resolve.
		//
		// TBD determine what pattern(s) emerge with clustered behavior
		// given that a connection to each broker in the cluster must
		// be maintained since there is no way to identify which broker
		// in the cluster is currently responsible for a given check...
		if c.connAttempts%ConfigRetryLimit == 0 {
			// Check configuration refresh -- TBD on if check refresh really needed or just find owner again for clustered
			c.Unlock()
			return nil, &connError{retry: false, err: errors.Errorf("max connection attempts (%d), check refresh", c.connAttempts)}
		}
	}
	c.Unlock()

	revHost := c.revConfig.ReverseURL.Host
	c.logger.Debug().Str("host", revHost).Msg("connecting")
	c.Lock()
	c.connAttempts++
	c.Unlock()
	dialer := &net.Dialer{Timeout: DialerTimeoutSeconds * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", c.revConfig.BrokerAddr.String(), c.revConfig.TLSConfig)
	if err != nil {
		if ne, ok := err.(*net.OpError); ok {
			if ne.Timeout() {
				return nil, &connError{retry: ne.Temporary(), err: errors.Wrapf(err, "timeout connecting to %s", revHost)}
			}
		}
		return nil, &connError{retry: true, err: errors.Wrapf(err, "connecting to %s", revHost)}
	}
	c.logger.Info().Str("host", revHost).Msg("connected")

	if err := conn.SetDeadline(time.Now().Add(CommTimeoutSeconds * time.Second)); err != nil {
		c.logger.Warn().Err(err).Msg("setting connection deadline")
	}
	introReq := "REVERSE " + c.revConfig.ReverseURL.Path
	if c.revConfig.ReverseURL.Fragment != "" {
		introReq += "#" + c.revConfig.ReverseURL.Fragment // reverse secret is placed here when reverse url is parsed
	}
	c.logger.Debug().Msg(fmt.Sprintf("sending intro '%s'", introReq))
	if _, err := fmt.Fprintf(conn, "%s HTTP/1.1\r\n\r\n", introReq); err != nil {
		if err != nil {
			c.logger.Error().Err(err).Msg("sending intro")
			return nil, &connError{retry: true, err: errors.Wrapf(err, "unable to write intro to %s", revHost)}
		}
	}

	c.Lock()
	c.State = StateConnIdle
	// reset timeouts after successful (re)connection
	c.commTimeouts = 0
	c.Unlock()

	return conn, nil
}

// getNextDelay for failed connection attempts
func (c *Connection) getNextDelay(currDelay time.Duration) time.Duration {
	maxDelay := MaxDelaySeconds * time.Second

	if currDelay == maxDelay {
		return currDelay
	}

	delay := currDelay

	if delay < maxDelay {
		drift := rand.Intn(MaxDelayStep-MinDelayStep) + MinDelayStep
		delay += time.Duration(drift) * time.Second
	}

	if delay > maxDelay {
		delay = maxDelay
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
