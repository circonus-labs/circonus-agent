// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"crypto/tls"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-gometrics/api"
	"github.com/rs/zerolog"
)

// metricStates holds the status of known metrics persisted to metrics.json in defaults.StatePath
type metricStates map[string]string

// Check exposes the check bundle management interface
type Check struct {
	lastRefresh         time.Time
	refreshTTL          time.Duration
	manage              bool
	bundle              *api.CheckBundle
	metricStates        metricStates
	updateMetricStates  bool
	activeMetrics       metricStates
	updateActiveMetrics bool
	revConfig           *ReverseConfig
	client              API
	logger              zerolog.Logger
	stateFile           string
	statePath           string
	sync.Mutex
}

// ReverseConfig contains the reverse configuration for the check
type ReverseConfig struct {
	ReverseURL *url.URL
	BrokerAddr *net.TCPAddr
	TLSConfig  *tls.Config
	BrokerID   string
}

const (
	// NOTE: TBD, possibly make retries and response time configurable
	brokerMaxRetries      = 5
	brokerMaxResponseTime = 500 * time.Millisecond
	brokerActiveStatus    = "active"
	activeMetricStatus    = "active"
)
