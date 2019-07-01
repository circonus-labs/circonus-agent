// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"time"

	"github.com/pkg/errors"
)

// sendMetricData frames and sends data (in chunks <= maxPayloadLen) to broker
func (c *Connection) sendMetricData(r io.Writer, channelID uint16, data *[]byte, cmdStart time.Time) error {
	sendStart := time.Now()
	empty := []byte("{}")
	if data == nil {
		data = &empty
	}
	c.logger.Debug().Uint16("channel_id", channelID).Str("duration", time.Since(cmdStart).String()).Msg("start CONNECT command response")

	sentBytes := 0
	for offset := 0; offset < len(*data); {
		buff := make([]byte, int(math.Min(float64(len((*data)[offset:])), float64(c.maxPayloadLen))))
		copy(buff, (*data)[offset:])
		frame := buildFrame(channelID, false, buff)
		c.logger.Debug().
			Uint16("channel_id", channelID).
			Str("frame_hdr", fmt.Sprintf("%#v", frame[0:6])).
			Int("frame_size", len(frame)).
			Int("payload_len", len(buff)).
			Msg("metric payload frame")

		if conn, ok := r.(*tls.Conn); ok {
			if err := conn.SetDeadline(time.Now().Add(c.commTimeout)); err != nil {
				c.logger.Warn().Err(err).Msg("setting conn deadline")
			}
		}

		sent, err := r.Write(frame)
		if err != nil {
			return errors.Wrap(err, "writing metric data")
		}
		offset += sent
		sentBytes = offset
	}

	c.logger.Debug().Uint16("channel_id", channelID).Str("duration", time.Since(sendStart).String()).Int("bytes", sentBytes).Msg("metric data sent")

	if c.maxRequests != -1 && channelID > uint16(c.maxRequests) {
		if conn, ok := r.(*tls.Conn); ok {
			c.logger.Info().Uint16("channel_id", channelID).Int("max", c.maxRequests).Msg("resetting connection")
			conn.Close()
		}
	}

	return nil
}

// fetchMetricData sends the command arguments to the local agent
func (c *Connection) fetchMetricData(request *[]byte, channelID uint16) (*[]byte, error) {
	fetchStart := time.Now()
	conn, err := net.DialTimeout("tcp", c.agentAddress, c.dialerTimeout)
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
	if err := conn.SetDeadline(time.Now().Add(c.metricTimeout)); err != nil {
		c.logger.Warn().Err(err).Msg("setting conn deadline")
	}

	numBytes, err := conn.Write(*request)
	if err != nil {
		return nil, errors.Wrap(err, "writing metric request")
	}
	if numBytes != len(*request) {
		c.logger.Warn().
			Int("written_bytes", numBytes).
			Int("request_len", len(*request)).
			Msg("Mismatch")
	}

	data, err := ioutil.ReadAll(conn)
	if err != nil {
		return nil, errors.Wrap(err, "reading metric data")
	}

	c.logger.Debug().Uint16("channel_id", channelID).Str("duration", time.Since(fetchStart).String()).Int("bytes", len(data)).Msg("fetched metrics")

	return &data, nil
}
