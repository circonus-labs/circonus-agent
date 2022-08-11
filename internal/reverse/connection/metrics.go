// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package connection

import (
	"crypto/tls"
	"fmt"
	"io"
	"math"
	"net"
	"time"
)

// sendMetricData frames and sends data (in chunks <= maxPayloadLen) to broker.
func (c *Connection) sendMetricData(r io.Writer, channelID uint16, data *[]byte, cmdStart time.Time) error {
	sendStart := time.Now()
	empty := []byte("{}")
	if data == nil {
		data = &empty
	}
	c.logger.Debug().Uint16("channel_id", channelID).Str("duration", time.Since(cmdStart).String()).Msg("start CONNECT command response")

	sentBytes := 0
	for offset := 0; offset < len(*data); {
		buff := make([]byte, int(math.Min(float64(len((*data)[offset:])), float64(MaxPayloadLen))))
		copy(buff, (*data)[offset:])
		frame := buildFrame(channelID, false, buff)
		c.logger.Debug().
			Uint16("channel_id", channelID).
			Str("frame_hdr", fmt.Sprintf("%#v", frame[0:6])).
			Int("frame_size", len(frame)).
			Int("payload_len", len(buff)).
			Msg("metric payload frame")

		if conn, ok := r.(*tls.Conn); ok {
			if err := conn.SetDeadline(time.Now().Add(CommTimeoutSeconds * time.Second)); err != nil {
				c.logger.Warn().Err(err).Msg("setting connection deadline")
			}
		}

		sent, err := r.Write(frame)
		if err != nil {
			return fmt.Errorf("writing metric data: %w", err)
		}
		offset += sent
		sentBytes = offset
	}

	c.logger.Debug().Uint16("channel_id", channelID).Str("duration", time.Since(sendStart).String()).Int("bytes", sentBytes).Msg("metric data sent")

	if MaxRequests != -1 && int(channelID) > MaxRequests {
		if conn, ok := r.(*tls.Conn); ok {
			c.logger.Info().Uint16("channel_id", channelID).Int("max", MaxRequests).Msg("resetting connection")
			conn.Close()
		}
	}

	return nil
}

// fetchMetricData sends the command arguments to the local agent.
func (c *Connection) fetchMetricData(request *[]byte, channelID uint16) (*[]byte, error) {
	fetchStart := time.Now()
	conn, err := net.DialTimeout("tcp", c.agentAddress, DialerTimeoutSeconds*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connecting to agent for metrics: %w", err)
	}
	defer conn.Close()

	// set total transaction timeout (agent is local...).
	// complete, roundtrip transaction to get metrics
	// should take *less* than polling interval
	// with graph/dashboard _play_, metrics will go
	// back to broker as fast as possible, gated by
	// plugin execution speed
	if err = conn.SetDeadline(time.Now().Add(MetricTimeoutSeconds * time.Second)); err != nil {
		c.logger.Warn().Err(err).Msg("setting connection deadline")
	}

	numBytes, err := conn.Write(*request)
	if err != nil {
		return nil, fmt.Errorf("writing metric request: %w", err)
	}
	if numBytes != len(*request) {
		c.logger.Warn().
			Int("written_bytes", numBytes).
			Int("request_len", len(*request)).
			Msg("Mismatch")
	}

	data, err := io.ReadAll(conn)
	if err != nil {
		return nil, fmt.Errorf("reading metric data: %w", err)
	}

	c.logger.Debug().Uint16("channel_id", channelID).Str("duration", time.Since(fetchStart).String()).Int("bytes", len(data)).Msg("fetched metrics")

	return &data, nil
}
