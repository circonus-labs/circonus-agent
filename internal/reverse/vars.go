// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	"crypto/tls"
	"net/url"
	"sync"
	"time"

	tomb "gopkg.in/tomb.v2"

	"github.com/circonus-labs/circonus-agent/internal/check"
	"github.com/rs/zerolog"
)

// Connection defines a reverse connection
type Connection struct {
	agentAddress  string
	check         *check.Check
	checkCID      string
	cmdCh         chan *noitCommand
	commTimeout   time.Duration
	conn          *tls.Conn
	connAttempts  int
	delay         time.Duration
	dialerTimeout time.Duration
	enabled       bool
	logger        zerolog.Logger
	maxDelay      time.Duration
	metricTimeout time.Duration
	reverseURL    *url.URL
	t             tomb.Tomb
	tlsConfig     *tls.Config
	sync.Mutex
}

// noitHeader defines the header received from the noit/broker
type noitHeader struct {
	channelID  uint16
	isCommand  bool
	payloadLen uint32
}

// noitPacket defines the header + the payload (described by the header) received from the noit/broker
type noitPacket struct {
	header  *noitHeader
	payload []byte
}

// noitCommand is the encapsulation of the header+payload for a single command or
// a command + request
type noitCommand struct {
	channelID uint16
	command   string
	request   []byte
}

// connError returned from connect(), adds flag indicating whether the error is
// a warning or fatal.
type connError struct {
	err   error
	fatal bool
}

const (
	// NOTE: TBD, make some of these user-configurable
	commTimeoutSeconds    = 65        // seconds, when communicating with noit
	dialerTimeoutSeconds  = 15        // seconds, establishing connection
	metricTimeoutSeconds  = 50        // seconds, when communicating with agent
	maxPayloadLen         = 65529     // max unsigned short - 6 (for header)
	maxConnRetry          = 10        // max times to retry a persistently failing connection
	configRetryLimit      = 5         // if failed attempts > threshold, force reconfig
	maxDelaySeconds       = 60        // maximum amount of delay between attempts
	minDelayStep          = 1         // minimum seconds to add on retry
	maxDelayStep          = 20        // maximum seconds to add on retry
	noitCmdConnect        = "CONNECT" // command from noit/broker
	brokerMaxRetries      = 5
	brokerMaxResponseTime = 500 * time.Millisecond
	brokerActiveStatus    = "active"
)
