// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"time"

	"github.com/pkg/errors"
)

// sendMetricData frames and sends data (in chunks <= maxPayloadLen) to broker
func (c *Connection) sendMetricData(r io.Writer, channelID uint16, data *[]byte) error {
	if data == nil {
		data = &emptyMetricsResponse
	}
	for offset := 0; offset < len(*data); {
		buff := make([]byte, int(math.Min(float64(len((*data)[offset:])), float64(maxPayloadLen))))
		copy(buff, (*data)[offset:])
		frame := buildFrame(channelID, false, buff)
		c.logger.Debug().
			Str("frame_hdr", fmt.Sprintf("%#v", frame[0:6])).
			Int("frame_size", len(frame)).
			Int("payload_len", len(buff)).
			Msg("metric payload frame")
		if c.conn != nil {
			c.conn.SetDeadline(time.Now().Add(c.commTimeout))
		}
		sentBytes, err := r.Write(frame)
		if err != nil {
			return errors.Wrap(err, "writing metric data")
		}
		offset += sentBytes
	}

	c.logger.Debug().
		Int("bytes", len(*data)).
		Msg("metric data sent")

	return nil
}

// fetchMetricData sends the command arguments to the local agent
func (c *Connection) fetchMetricData(request *[]byte) (*[]byte, error) {
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
	conn.SetDeadline(time.Now().Add(c.metricTimeout))

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

	return &data, nil
}
