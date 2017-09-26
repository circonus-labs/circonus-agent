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
	"math/rand"
	"net"
	"time"

	"github.com/pkg/errors"
)

const (
	noitCmdConnect = "CONNECT"
)

// connect to broker via w/tls and send initial introduction to start reverse
// NOTE: all reverse connections require tls
func (c *Connection) connect() error {
	c.logger.Info().Str("host", c.reverseURL.Host).Msg("Connecting")

	c.connAttempts++
	dialer := &net.Dialer{Timeout: c.dialerTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", c.reverseURL.Host, c.tlsConfig)
	if err != nil {
		return err
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
func (c *Connection) processCommands() error {
	defer c.conn.Close()

	for {
		if c.isShuttingDown() {
			return nil
		}

		nc, err := c.getCommandFromBroker()
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

		// send the metrics received from the local agent back to the broker
		if err := c.sendMetricData(nc.channelID, data); err != nil {
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

//
// broker connection
//

// getCommandFromBroker reads a command and optional request from broker
func (c *Connection) getCommandFromBroker() (*noitCommand, error) {
	nc := &noitCommand{}

	cmdPkt, err := c.getDataFromBroker()
	if err != nil {
		return nil, err
	}

	if !cmdPkt.header.isCommand {
		c.logger.Warn().
			Str("cmd_header", fmt.Sprintf("%#v", cmdPkt.header)).
			Str("cmd_payload", string(cmdPkt.payload)).
			Msg("expected command")
		return nil, nil
	}

	nc.channelID = cmdPkt.header.channelID
	nc.command = string(cmdPkt.payload)

	if nc.command == noitCmdConnect { // connect command requires a request
		reqPkt, err := c.getDataFromBroker()
		if err != nil {
			return nil, err
		}

		if reqPkt.header.isCommand {
			c.logger.Warn().
				Str("cmd_header", fmt.Sprintf("%#v", cmdPkt.header)).
				Str("cmd_payload", string(cmdPkt.payload)).
				Str("req_header", fmt.Sprintf("%#v", reqPkt.header)).
				Str("req_payload", string(reqPkt.payload)).
				Msg("expected request")
			return nil, nil
		}

		nc.request = reqPkt.payload
	}

	return nc, nil
}

// getDataFromBroker reads (header + payload) from broker
func (c *Connection) getDataFromBroker() (*noitPacket, error) {
	hdr, err := c.readHeader()
	if err != nil {
		return nil, err
	}

	if hdr.payloadLen > maxPayloadLen {
		return nil, errors.Errorf("received oversized frame (%d len)", hdr.payloadLen) // restart the connection
	}

	payload, err := c.readPayload(hdr.payloadLen)
	if err != nil {
		return nil, err
	}

	c.logger.Debug().
		Uint16("channel", hdr.channelID).
		Bool("is_command", hdr.isCommand).
		Uint32("payload_len", hdr.payloadLen).
		Str("payload", fmt.Sprintf("%s", string(payload))).
		Msg("data from broker")

	return &noitPacket{
		header:  hdr,
		payload: payload,
	}, nil
}

// readHeader reads 6 bytes from the broker connection
func (c *Connection) readHeader() (*noitHeader, error) {
	data, err := c.readPayload(6)
	if err != nil {
		return nil, err
	}

	encodedChannelID := binary.BigEndian.Uint16(data)
	hdr := &noitHeader{
		channelID:  encodedChannelID & 0x7fff,
		isCommand:  encodedChannelID&0x8000 > 0,
		payloadLen: binary.BigEndian.Uint32(data[2:]),
	}

	return hdr, nil
}

// readPayload consumes n bytes from the broker connection
func (c *Connection) readPayload(size uint32) ([]byte, error) {
	if size == 0 {
		return []byte{}, nil
	}
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
	c.conn.SetDeadline(time.Now().Add(c.commTimeout))

	buff := make([]byte, size)
	lr := io.LimitReader(c.conn, size)

	n, err := lr.Read(buff)
	if n == 0 && err != nil {
		return nil, err
	}

	// c.logger.Debug().Int("bytes", n).Err(err).Msg("read")

	return buff, nil
}
