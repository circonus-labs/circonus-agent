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
	statusActiveMetric    string
	statusActiveBroker    string
	brokerMaxResponseTime time.Duration
	brokerMaxRetries      int
	bundle                *api.CheckBundle
	client                API
	lastRefresh           time.Time
	logger                zerolog.Logger
	manage                bool
	metricStates          *metricStates
	metricStateUpdate     bool
	refreshTTL            time.Duration
	revConfig             *ReverseConfig
	stateFile             string
	statePath             string
	sync.Mutex
}

// ReverseConfig contains the reverse configuration for the check
type ReverseConfig struct {
	BrokerAddr *net.TCPAddr
	BrokerID   string
	ReverseURL *url.URL
	TLSConfig  *tls.Config
}
