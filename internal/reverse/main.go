// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// Connection defines a reverse connection
type Connection struct {
	commTimeout   time.Duration
	dialerTimeout time.Duration
	metricTimeout time.Duration
	logger        zerolog.Logger
	shutdown      bool
	conn          *tls.Conn
}

type header struct {
	channelID  uint16
	isCommand  bool
	payloadLen uint32
}

const (
	// NOTE: TBD, make some of these user-configurable
	commTimeoutSeconds   = 65    // seconds, when communicating with noit
	dialerTimeoutSeconds = 15    // seconds, establishing connection
	metricTimeoutSeconds = 50    // seconds, when communicating with agent
	maxPayloadLen        = 65529 // max unsigned short - 6 (for header)
	maxConnRetry         = 10    // max times to retry a persistently failing connection
	configRetryLimit     = 5     // if failed attempts > threshold, force reconfig
)

// var (
// 	commTimeout   = commTimeoutSeconds * time.Second
// 	dialerTimeout = dialerTimeoutSeconds * time.Second
// 	metricTimeout = metricTimeoutSeconds * time.Second
// 	logger        zerolog.Logger
// )

// New creates a new connection
func New() *Connection {
	if !viper.GetBool(config.KeyReverse) {
		log.Info().Msg("Reverse disabled, not starting")
		return nil
	}

	c := Connection{
		commTimeout:   commTimeoutSeconds * time.Second,
		dialerTimeout: dialerTimeoutSeconds * time.Second,
		metricTimeout: metricTimeoutSeconds * time.Second,
		logger:        log.With().Str("pkg", "reverse").Logger(),
		shutdown:      true,
	}

	return &c
}

// Start reverse connection to the broker
func (c *Connection) Start() error {

	c.logger.Info().Msg("Setting up reverse connections")

	attempt := 1
	backoffs := []time.Duration{
		2 * time.Second,
		4 * time.Second,
		6 * time.Second,
		8 * time.Second,
		16 * time.Second,
		32 * time.Second,
		60 * time.Second,
	}
	maxAttempts := len(backoffs) - 1

	agentAddress := strings.Replace(viper.GetString(config.KeyListen), "0.0.0.0", "localhost", -1)

	// catch initial errors during configuration
	var (
		err        error
		reverseURL *url.URL
		tlsConfig  *tls.Config
	)
	reverseURL, tlsConfig, err = c.configure()
	if err != nil {
		return err
	}

	c.logger.Info().
		Str("check_bundle", viper.GetString(config.KeyReverseCID)).
		Str("rev_host", reverseURL.Hostname()).
		Str("rev_port", reverseURL.Port()).
		Str("rev_path", reverseURL.Path).
		Str("agent", agentAddress).
		Msg("Reverse configuration")

	ec := make(chan error)

	go func() {
		for { // allow for restarts
			if reverseURL == nil || attempt%configRetryLimit == 0 {
				c.logger.Info().
					Int("attempts", attempt).
					Msg("reconfig triggered")
				// Under normal circumstances the configuration for reverse is
				// non-volatile. There are, however, some situations where the
				// configuration must be rebuilt. (e.g. ip of broker changed,
				// check changed to use a different broker, broker certificate
				// changes, etc.) The majority of configuration based errors are
				// fatal, no attempt is made to resolve.
				reverseURL, tlsConfig, err = c.configure()
				if err != nil {
					ec <- errors.Wrap(err, "configuring reverse connection")
					return
				}
			}

			c.conn, err = c.connect(reverseURL, tlsConfig)
			if err != nil {
				if attempt >= maxConnRetry { // retry n times on connection attempt failures
					ec <- errors.Wrapf(err, "%d failed attempts", attempt)
					return
				}
				c.logger.Error().
					Err(err).
					Int("attempt", attempt).
					Msg("failed")
			} else {
				c.shutdown = false                  // indicate reverse connection is open
				attempt = 1                         // reset on successful connection
				c.reverse(reverseURL, agentAddress) // reconnect
			}

			// shutting down
			if c.shutdown {
				ec <- nil
				break
			}

			// backoff retry on each consecutive failure
			delay := backoffs[uint8(math.Min(float64(attempt-1), float64(maxAttempts)))]
			attempt++
			c.logger.Info().
				Str("delay", delay.String()).
				Int("attempt", attempt).
				Msg("connect retry")
			time.Sleep(delay)
		}
	}()

	// block until an error is recieved or some other component exits
	return <-ec
}

// Stop the reverse connection
func (c *Connection) Stop() {
	if !c.shutdown && c.conn != nil {
		c.logger.Debug().Msg("Stopping reverse connection")
		c.shutdown = true
		err := c.conn.Close()
		if err != nil {
			c.logger.Warn().Err(err).Msg("Closing reverse connection")
		}
	}
}

func (c *Connection) connect(reverseURL *url.URL, tlsConfig *tls.Config) (*tls.Conn, error) {
	c.logger.Info().
		Str("host", reverseURL.Host).
		Msg("Connecting")

	dialer := &net.Dialer{Timeout: c.dialerTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", reverseURL.Host, tlsConfig)
	if err != nil {
		return nil, err
	}

	conn.SetDeadline(time.Now().Add(c.commTimeout))
	introReq := "REVERSE " + reverseURL.Path
	if reverseURL.Fragment != "" {
		introReq += "#" + reverseURL.Fragment // reverse secret is placed here when reverse url is parsed
	}
	c.logger.Debug().Msg(fmt.Sprintf("Sending intro '%s'", introReq))
	if _, err := fmt.Fprintf(conn, "%s HTTP/1.1\r\n\r\n", introReq); err != nil {
		if err != nil {
			c.logger.Error().
				Err(err).
				Str("host", reverseURL.Host).
				Msg("Unable to write intro")
			return nil, err
		}
	}

	return conn, nil
}

func (c *Connection) reverse(reverseURL *url.URL, agentAddress string) {
	defer c.conn.Close()

	cmd := []byte{}
	arg := []byte{}
	for {

		// set deadline for comms with broker before each read/write
		c.conn.SetDeadline(time.Now().Add(c.commTimeout))

		hdr, err := c.readHeader()
		if err != nil {
			c.logger.Error().
				Err(err).
				Msg("reading header")
			return // restart the connection
		}

		if hdr.payloadLen > maxPayloadLen {
			c.logger.Warn().
				Uint32("payload_len", hdr.payloadLen).
				Msg("Oversized frame, resetting connection")
			return // restart the connection
		}

		msg, err := c.readMessage(hdr.payloadLen)
		if err != nil {
			c.logger.Error().
				Err(err).
				Msg("reading message")
			return // restart the connection
		}

		if hdr.isCommand {
			cmd = msg
			c.logger.Debug().
				Str("cmd", string(cmd)).
				Msg("received command")
		} else {
			arg = msg
			c.logger.Debug().
				Str("arg", string(arg)).
				Msg("received request")
		}

		// From the perspective of a "client" it is ambiguous whether the CLOSE,
		// RESET, and SHUTDOWN commands - when received by the client, from
		// the noit - pertain to the agent (NAD|circonus-agent) connection (local) or
		// the noit connection itself (remote).
		switch string(cmd) {
		case "CONNECT":
			// ignore the first time through, there is an argument coming
			/// next (the request to send to the agent, e.g. 'GET / ...')
			if len(arg) > 0 {
				c.logger.Debug().
					Str("cmd", string(cmd)).
					Str("arg", string(arg)).
					Msg("processing command")
				data, err := c.fetchMetricData(agentAddress, arg)
				if err != nil {
					// log the error and respond with no metrics
					c.logger.Error().
						Err(err).
						Msg("fetching metric data")
					data = []byte("{}")
				}
				if err := c.sendMetricData(data, hdr.channelID, arg); err != nil {
					if err != nil {
						c.logger.Error().
							Err(err).
							Msg("sending metric data")
						return // restart the connection
					}
				}
				cmd = []byte{}
				arg = []byte{}
			}
		case "CLOSE":
			fallthrough
		case "RESET":
			fallthrough
		case "SHUTDOWN":
			// logger.Debug().
			// 	Str("cmd", string(cmd)).
			// 	Uint16("channel_id", channelID).
			// 	Msg("ignoring command")
			cmd = []byte{}
		default:
			c.logger.Warn().
				Str("cmd", string(cmd)).
				Uint16("channel_id", hdr.channelID).
				Msg("unknown command")
		}
	}
}

// sendMetricData frames and sends data (in chunks <= maxPayloadLen) to broker
func (c *Connection) sendMetricData(data []byte, channelID uint16, request []byte) error {
	for offset := 0; offset < len(data); {
		buff := make([]byte, int(math.Min(float64(len(data[offset:])), float64(maxPayloadLen))))
		copy(buff, data[offset:])
		sentBytes, err := c.conn.Write(c.buildFrame(channelID, buff))
		if err != nil {
			return errors.Wrap(err, "writing metric data")
		}
		offset += sentBytes
	}

	c.logger.Debug().
		Int("bytes", len(data)).
		Msg("metric data sent")

	return nil
}

// buildFrame creates a frame to send to broker.
// recipe:
// bytes 1-6 header
//      2 bytes command
//      4 bytes length of data
// bytes 7-n are data, where 0 < n <= maxPayloadLen
func (c *Connection) buildFrame(channelID uint16, data []byte) []byte {
	frame := make([]byte, len(data)+6)

	copy(frame[6:], data)
	binary.BigEndian.PutUint16(frame[0:], channelID&0x7fff)
	binary.BigEndian.PutUint32(frame[2:], uint32(len(data)))

	c.logger.Debug().
		Str("frame_hdr", fmt.Sprintf("%#v", frame[0:6])).
		Int("frame_size", len(frame)).
		Int("payload_len", len(data)).
		Msg("built payload frame")
	return frame
}

// fetchMetricData sends the command arguments to the local agent
func (c *Connection) fetchMetricData(agentAddress string, request []byte) ([]byte, error) {
	conn, err := net.DialTimeout("tcp", agentAddress, c.dialerTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "connecting to agent for metrics")
	}
	defer conn.Close()

	// set total transaction timeout (agent is local...).
	// complete, roundtrip transaction to get metrics
	// should take *less* than polling interval
	// with graph/dashboard _play_, metrics will go
	// back to broker as fast as possible, gated by
	// plugin execution speed
	conn.SetDeadline(time.Now().Add(c.metricTimeout))

	numBytes, err := conn.Write(request)
	if err != nil {
		return nil, errors.Wrap(err, "writing metric request")
	}
	if numBytes != len(request) {
		c.logger.Warn().
			Int("written_bytes", numBytes).
			Int("request_len", len(request)).
			Msg("Mismatch")
	}

	data, err := ioutil.ReadAll(conn)
	if err != nil {
		return nil, errors.Wrap(err, "reading metric data")
	}

	return data, nil
}

// readHeader reads 6 bytes from the broker connection
func (c *Connection) readHeader() (header, error) {
	var hdr header
	data, err := c.readMessage(6)
	if err != nil {
		return hdr, err
	}

	encodedChannelID := binary.BigEndian.Uint16(data)
	hdr.channelID = encodedChannelID & 0x7fff
	hdr.isCommand = encodedChannelID&0x8000 > 0
	hdr.payloadLen = binary.BigEndian.Uint32(data[2:])

	c.logger.Debug().
		Str("frame", fmt.Sprintf("%#v", data)).
		Uint16("channel", hdr.channelID).
		Bool("is_command", hdr.isCommand).
		Uint32("payload_len", hdr.payloadLen).
		Msg("read header")

	return hdr, nil
}

// readMessage reads n bytes from the broker connection
func (c *Connection) readMessage(size uint32) ([]byte, error) {
	data, err := c.readBytes(int64(size))
	if err != nil {
		return nil, err
	}
	if uint32(len(data)) != size {
		return nil, errors.Errorf("undersized read, expected %d received %d (%#v)", size, len(data), data)
	}
	return data, nil
}

// readBytes attempts to reads <size> bytes from broker connection.
func (c *Connection) readBytes(size int64) ([]byte, error) {
	buff := make([]byte, size)
	lr := io.LimitReader(c.conn, size)

	n, err := lr.Read(buff)
	if n == 0 && err != nil {
		return nil, err
	}

	c.logger.Debug().Int("bytes", n).Err(err).Msg("read")

	return buff, nil
}
