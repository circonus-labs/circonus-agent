// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	"sync"
	"time"

	tomb "gopkg.in/tomb.v2"

	"github.com/circonus-labs/circonus-agent/internal/check"
	"github.com/rs/zerolog"
)

// Connection defines a reverse connection
type Connection struct {
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
	metricTimeout    time.Duration
	minDelayStep     int
	revConfig        check.ReverseConfig
	sync.Mutex
	t tomb.Tomb
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
}
