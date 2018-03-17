// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"crypto/tls"
	"net/url"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-gometrics/api"
	"github.com/rs/zerolog"
)

type metricState struct {
	active     bool
	metricType string
}

// Check exposes the check bundle management interface
type Check struct {
	lastRefresh  time.Time
	refreshTTL   time.Duration
	manage       bool
	bundle       *api.CheckBundle
	checkMetrics map[string]api.CheckBundleMetric
	knownMetrics map[string]string
	revConfig    *ReverseConfig
	client       API
	logger       zerolog.Logger
	sync.Mutex
}

// ReverseConfig contains the reverse configuration for the check
type ReverseConfig struct {
	ReverseURL *url.URL
	TLSConfig  *tls.Config
	BrokerID   string
}

const (
	// NOTE: TBD, possibly make retries and response time configurable
	brokerMaxRetries      = 5
	brokerMaxResponseTime = 500 * time.Millisecond
	brokerActiveStatus    = "active"
)
