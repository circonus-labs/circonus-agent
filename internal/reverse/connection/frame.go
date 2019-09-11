// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package connection

import (
	"crypto/tls"
	"encoding/binary"
	"io"
	"time"

	"github.com/pkg/errors"
)

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

// readFrameFromBroker reads a frame(header + payload) from broker
func (c *Connection) readFrameFromBroker(r io.Reader) (*noitFrame, error) {
	if conn, ok := r.(*tls.Conn); ok {
		if err := conn.SetDeadline(time.Now().Add(CommTimeoutSeconds * time.Second)); err != nil {
			c.logger.Warn().Err(err).Msg("setting connection deadline")
		}
	}
	hdr, err := readFrameHeader(r)
	if err != nil {
		return nil, err
	}

	if hdr.payloadLen > MaxPayloadLen {
		return nil, errors.Errorf("received oversized frame (%d len)", hdr.payloadLen) // restart the connection
	}

	if conn, ok := r.(*tls.Conn); ok {
		if err := conn.SetDeadline(time.Now().Add(CommTimeoutSeconds * time.Second)); err != nil {
			c.logger.Warn().Err(err).Msg("setting connection deadline")
		}
	}
	payload, err := readFramePayload(r, hdr.payloadLen)
	if err != nil {
		return nil, err
	}

	c.logger.Debug().
		Uint16("channel_id", hdr.channelID).
		Bool("is_command", hdr.isCommand).
		Uint32("payload_len", hdr.payloadLen).
		Str("payload", string(payload)).
		Msg("data from broker")

	return &noitFrame{
		header:  hdr,
		payload: payload,
	}, nil
}

// readFrameHeader reads 6 bytes from the broker connection
func readFrameHeader(r io.Reader) (*noitHeader, error) {
	hdrSize := 6

	data, err := readBytes(r, int64(hdrSize))
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

// readFramePayload consumes n bytes from the broker connection
func readFramePayload(r io.Reader, size uint32) ([]byte, error) {
	data, err := readBytes(r, int64(size))
	if err != nil {
		return nil, err
	}
	return data, nil
}

// readBytes attempts to reads <size> bytes from broker connection.
func readBytes(r io.Reader, size int64) ([]byte, error) {
	if size == 0 {
		return []byte{}, nil
	}
	buff := make([]byte, 0, size)
	lr := io.LimitReader(r, size)

	n, err := lr.Read(buff[:cap(buff)])
	if n == 0 && err != nil {
		return nil, err
	}

	// dealing with expected sizes
	if int64(n) != size {
		sz := 30 // 30 is arbitrary, max amount of a larger buffer wanted in a log message
		if n < sz {
			sz = n
		}
		return nil, errors.Errorf("invalid read, expected bytes %d got %d (%#v = %s)", size, n, buff[0:sz], string(buff[0:sz]))
	}

	return buff[:n], nil
}
