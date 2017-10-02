// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package statsd

import (
	"context"
	"net"
	"regexp"
	"sync"

	cgm "github.com/circonus-labs/circonus-gometrics"
	"github.com/rs/zerolog"
	"gopkg.in/tomb.v2"
)

// Server defines a statsd server
type Server struct {
	ctx                   context.Context
	disabled              bool
	address               *net.UDPAddr
	hostMetrics           *cgm.CirconusMetrics
	hostMetricsmu         sync.Mutex
	groupMetrics          *cgm.CirconusMetrics
	groupMetricsmu        sync.Mutex
	logger                zerolog.Logger
	hostPrefix            string
	hostCategory          string
	groupCID              string
	groupPrefix           string
	groupCounterOp        string
	groupGaugeOp          string
	groupSetOp            string
	metricRegex           *regexp.Regexp
	metricRegexGroupNames []string
	apiKey                string
	apiApp                string
	apiURL                string
	apiCAFile             string
	debugCGM              bool
	listener              *net.UDPConn
	packetCh              chan []byte
	t                     tomb.Tomb
}

const (
	maxPacketSize   = 1472
	packetQueueSize = 1000
	destHost        = "host"
	destGroup       = "group"
	destIgnore      = "ignore"
)
