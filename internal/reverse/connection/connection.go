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
	"github.com/maier/go-appstats"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

type Connection struct {
	sync.Mutex
	LastRequestTime *time.Time
	revConfig       check.ReverseConfig
	State           string
	agentAddress    string
	logger          zerolog.Logger
	delay           time.Duration
	commTimeouts    int
	connAttempts    int
	maxConnRetry    int
}

// command contains details of the command received from the broker.
type command struct {
	start     time.Time
	metrics   *[]byte
	err       error
	name      string
	request   []byte
	channelID uint16
	ignore    bool
	fatal     bool
	reset     bool
}

// noitHeader defines the header received from the noit/broker.
type noitHeader struct {
	channelID  uint16
	isCommand  bool
	payloadLen uint32
}

// noitFrame defines the header + the payload (described by the header) received from the noit/broker.
type noitFrame struct {
	header  *noitHeader
	payload []byte
}

// connError returned from connect(), adds flag indicating whether to retry.
type connError struct {
	err   error
	retry bool
}

type OpError struct {
	OrigErr      error
	Err          string
	Fatal        bool
	RefreshCheck bool
}

func (e *OpError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err != "" {
		return e.Err + ": (" + e.OrigErr.Error() + ")"
	}
	return e.OrigErr.Error()
}

const (
	StateConnActive = "CONN_ACTIVE" // connected, broker requesting metrics
	StateConnIdle   = "CONN_IDLE"   // connected, no requests
	StateNew        = "NEW"         // new, no attempt to connect yet
	StateError      = "ERROR"       // connection is erroring
	CommandConnect  = "CONNECT"     // Connect command, must be followed by a request payload
	CommandReset    = "RESET"       // Reset command, resets the connection

	// NOTE: TBD, make some of these user-configurable.
	CommTimeoutSeconds   = 10    // seconds, when communicating with noit
	DialerTimeoutSeconds = 15    // seconds, establishing connection
	MetricTimeoutSeconds = 50    // seconds, when communicating with agent
	MaxDelaySeconds      = 10    // maximum amount of delay between attempts
	MaxRequests          = -1    // max requests from broker before resetting connection, -1 = unlimited
	MaxPayloadLen        = 65529 // max unsigned short - 6 (for header)
	MaxCommTimeouts      = 6     // multiply by commTimeout, ensure >(broker polling interval) otherwise conn reset loop
	MinDelayStep         = 1     // minimum seconds to add on retry
	MaxDelayStep         = 7     // maximum seconds to add on retry
	ConfigRetryLimit     = 3     // if failed attempts > limit, force check reconfig (see if broker configuration changed)
)

func New(parentLogger zerolog.Logger, agentAddress string, cfg *check.ReverseConfig) (*Connection, error) {
	if agentAddress == "" {
		return nil, fmt.Errorf("invalid agent address (empty)") //nolint:goerr113
	}
	if cfg == nil {
		return nil, fmt.Errorf("invalid config (nil)") //nolint:goerr113
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

// Start the reverse connection to the broker.
func (c *Connection) Start(ctx context.Context) error {
	for {
		conn, cerr := c.connect(ctx)
		if cerr != nil {
			if aerr := appstats.SetString("reverse.last_connect_error", time.Now().String()); aerr != nil {
				c.logger.Warn().Err(aerr).Msg("setting app stat - last_connect_error")
			}
			if cerr.retry {
				c.logger.Warn().Err(cerr.err).Msg("retrying")
				continue
			}
			// c.logger.Error().Err(cerr.err).Msg("unable to establish reverse connection to broker")
			return &OpError{
				Err:          "unable to establish reverse connection to broker",
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
					if aerr := appstats.SetString("reverse.last_connect_reset", time.Now().String()); aerr != nil {
						c.logger.Warn().Err(aerr).Msg("setting app stat - last_connect_reset")
					}
					c.logger.Warn().Err(result.err).Int("timeouts", c.commTimeouts).Msg("resetting connection")
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
				cmdCancel()
				break // inner loop for check refresh
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
// NOTE: all reverse connections require tls.
func (c *Connection) connect(ctx context.Context) (*tls.Conn, *connError) {
	c.Lock()
	if c.connAttempts > 0 {
		if c.maxConnRetry != -1 && c.connAttempts >= c.maxConnRetry {
			c.Unlock()
			return nil, &connError{retry: false, err: fmt.Errorf("max broker connection attempts reached (%d of %d)", c.connAttempts, c.maxConnRetry)} //nolint:goerr113
		}

		c.logger.Info().
			Str("delay", c.delay.String()).
			Int("attempt", c.connAttempts).
			Msg("connect retry")

		time.Sleep(c.delay)
		select {
		case <-ctx.Done():
			c.Unlock()
			return nil, nil
		default:
		}
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
			return nil, &connError{retry: false, err: fmt.Errorf("max connection attempts (%d), check refresh", c.connAttempts)} //nolint:goerr113
		}

		if err := appstats.SetString("reverse.last_conn_retry", time.Now().String()); err != nil {
			c.logger.Warn().Err(err).Msg("setting app stat - last_conn_retry")
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
		if ne, ok := err.(*net.OpError); ok { //nolint:errorlint
			if ne.Timeout() {
				return nil, &connError{retry: ne.Temporary(), err: fmt.Errorf("timeout connecting to %s: %w", revHost, err)}
			}
		}
		return nil, &connError{retry: true, err: fmt.Errorf("connecting to %s: %w", revHost, err)}
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
		c.logger.Error().Err(err).Msg("sending intro")
		return nil, &connError{retry: true, err: fmt.Errorf("unable to write intro to %s: %w", revHost, err)}
	}

	if err := appstats.SetString("reverse.last_connect", time.Now().String()); err != nil {
		c.logger.Warn().Err(err).Msg("setting app stat - last_connect")
	}

	c.Lock()
	c.State = StateConnIdle
	// reset timeouts after successful (re)connection
	c.commTimeouts = 0
	c.Unlock()

	return conn, nil
}

// getNextDelay for failed connection attempts.
func (c *Connection) getNextDelay(currDelay time.Duration) time.Duration {
	maxDelay := MaxDelaySeconds * time.Second

	if currDelay == maxDelay {
		return time.Duration(MinDelayStep) * time.Second
	}

	delay := currDelay

	if delay < maxDelay {
		drift := rand.Intn(MaxDelayStep-MinDelayStep) + MinDelayStep //nolint:gosec
		delay += time.Duration(drift) * time.Second
	}

	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// resetConnectionAttempts on successful send/receive.
func (c *Connection) resetConnectionAttempts() {
	c.Lock()
	if c.connAttempts > 0 {
		c.delay = 1 * time.Second
		c.connAttempts = 0
	}
	c.Unlock()
}

// Error returns string representation of a connError.
func (e *connError) Error() string {
	return e.err.Error()
}
