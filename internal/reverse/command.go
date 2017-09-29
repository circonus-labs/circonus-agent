// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"
)

// getCommandFromBroker reads a command and optional request from broker
func (c *Connection) getCommandFromBroker(r io.Reader) (*noitCommand, error) {
	if c.shutdown() {
		return nil, nil
	}

	cmdPkt, err := c.getFrameFromBroker(r)
	if c.shutdown() {
		return nil, nil
	}
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

	nc := &noitCommand{
		channelID: cmdPkt.header.channelID,
		command:   string(cmdPkt.payload),
	}

	if nc.command == noitCmdConnect { // connect command requires a request
		reqPkt, err := c.getFrameFromBroker(r)
		if c.shutdown() {
			return nil, nil
		}
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

// getFrameFromBroker reads a frame(header + payload) from broker
func (c *Connection) getFrameFromBroker(r io.Reader) (*noitPacket, error) {
	if c.conn != nil {
		c.conn.SetDeadline(time.Now().Add(c.commTimeout))
	}
	hdr, err := readHeader(r)
	if err != nil {
		return nil, err
	}

	if hdr.payloadLen > maxPayloadLen {
		return nil, errors.Errorf("received oversized frame (%d len)", hdr.payloadLen) // restart the connection
	}

	if c.conn != nil {
		c.conn.SetDeadline(time.Now().Add(c.commTimeout))
	}
	payload, err := readPayload(r, hdr.payloadLen)
	if err != nil {
		return nil, err
	}

	c.logger.Debug().
		Uint16("channel", hdr.channelID).
		Bool("is_command", hdr.isCommand).
		Uint32("payload_len", hdr.payloadLen).
		Str("payload", string(payload)).
		Msg("data from broker")

	return &noitPacket{
		header:  hdr,
		payload: payload,
	}, nil
}

// readHeader reads 6 bytes from the broker connection
func readHeader(r io.Reader) (*noitHeader, error) {
	data, err := readPayload(r, 6)
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
func readPayload(r io.Reader, size uint32) ([]byte, error) {
	if size == 0 {
		return []byte{}, nil
	}
	data, err := readBytes(r, int64(size))
	if err != nil {
		return nil, err
	}
	return data, nil
}

// readBytes attempts to reads <size> bytes from broker connection.
func readBytes(r io.Reader, size int64) ([]byte, error) {
	buff := make([]byte, size)
	lr := io.LimitReader(r, size)

	n, err := lr.Read(buff)
	if n == 0 && err != nil {
		return nil, err
	}

	return buff, nil
}

// buildFrame creates a frame to send to broker.
// recipe:
// bytes 1-6 header
//      2 bytes channel id and command flag
//      4 bytes length of data
// bytes 7-n are data, where 0 < n <= maxPayloadLen
func buildFrame(channelID uint16, isCommand bool, payload []byte) []byte {
	frame := make([]byte, len(payload)+6)

	var cmdFlag uint16
	if isCommand {
		cmdFlag = 0x8000
	}

	copy(frame[6:], payload)
	binary.BigEndian.PutUint16(frame[0:], channelID&0x7fff|cmdFlag)
	binary.BigEndian.PutUint32(frame[2:], uint32(len(payload)))

	return frame
}
