// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"crypto/tls"
	stdlog "log"
	"net"
	"net/url"
	"path/filepath"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	cgm "github.com/circonus-labs/circonus-gometrics"
	"github.com/circonus-labs/circonus-gometrics/api"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
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

// Meta contains check id meta data
type Meta struct {
	BundleID string
	CheckIDs []string
}

// ReverseConfig contains the reverse configuration for the check
type ReverseConfig struct {
	BrokerAddr *net.TCPAddr
	BrokerID   string
	ReverseURL *url.URL
	TLSConfig  *tls.Config
}

// New returns a new check instance
func New(apiClient API) (*Check, error) {
	// NOTE: TBD, make broker max retries and response time configurable
	c := Check{
		brokerMaxResponseTime: 500 * time.Millisecond,
		brokerMaxRetries:      5,
		bundle:                nil,
		logger:                log.With().Str("pkg", "check").Logger(),
		manage:                false,
		metricStateUpdate:     false,
		refreshTTL:            time.Duration(0),
		statePath:             viper.GetString(config.KeyCheckMetricStateDir),
		statusActiveBroker:    "active",
		statusActiveMetric:    "active",
	}

	c.stateFile = filepath.Join(c.statePath, "metrics.json")

	isCreate := viper.GetBool(config.KeyCheckCreate)
	isManaged := viper.GetBool(config.KeyCheckEnableNewMetrics)
	isReverse := viper.GetBool(config.KeyReverse)
	cid := viper.GetString(config.KeyCheckBundleID)
	needCheck := false

	if isReverse || isManaged || (isCreate && cid == "") {
		needCheck = true
	}

	if !needCheck {
		c.logger.Info().Msg("check management disabled")
		return &c, nil // if we don't need a check, return a NOP object
	}

	if apiClient == nil {
		// create an API client
		cfg := &api.Config{
			TokenKey: viper.GetString(config.KeyAPITokenKey),
			TokenApp: viper.GetString(config.KeyAPITokenApp),
			URL:      viper.GetString(config.KeyAPIURL),
			Log:      stdlog.New(c.logger.With().Str("pkg", "check.api").Logger(), "", 0),
			Debug:    viper.GetBool(config.KeyDebugCGM),
		}
		client, err := api.New(cfg)
		if err != nil {
			return nil, errors.Wrap(err, "creating circonus api client")
		}
		apiClient = client
	}

	c.client = apiClient

	if isManaged {
		// preload the last known metric states so that states coming down
		// from the API when fetching the check bundle will be merged into
		// the known states since the fresh states have a higher precedence
		if ok, err := c.verifyStatePath(); ok {
			ms, err := c.loadState()
			if err != nil {
				c.logger.Error().Err(err).Msg("unable to load existing metric states, all metrics considered existing")
			} else {
				c.metricStates = ms
				c.logger.Debug().Interface("metric_states", len(*c.metricStates)).Msg("loaded metric states")
			}
		} else {
			if err != nil {
				c.logger.Error().Err(err).Msg("verify state path")
			}
			c.logger.Warn().Str("state_path", c.statePath).Msg("encountered state path issue(s), disabling check-enable-new-metrics")
			viper.Set(config.KeyCheckEnableNewMetrics, false)
			isManaged = false
			c.manage = false
		}
	}

	// fetch or create check bundle
	if err := c.setCheck(); err != nil {
		return nil, errors.Wrap(err, "unable to configure check")
	}

	// ensure a) the global check bundle id is set and b) it is set correctly to the
	// check bundle actually being used - need to do this even if the check was
	// created initially since user 'nobody' cannot create or update the configuration
	viper.Set(config.KeyCheckBundleID, c.bundle.CID)

	if !isManaged {
		return &c, nil
	}

	// check metrics refresh ttl
	ttl, err := time.ParseDuration(viper.GetString(config.KeyCheckMetricRefreshTTL))
	if err != nil {
		return nil, errors.Wrap(err, "parsing check metric refresh TTL")
	}
	if ttl == time.Duration(0) {
		ttl, err = time.ParseDuration(defaults.CheckMetricRefreshTTL)
		if err != nil {
			return nil, errors.Wrap(err, "parsing default check metric refresh TTL")
		}
	}

	c.refreshTTL = ttl
	c.manage = isManaged

	return &c, nil
}

// CheckMeta returns check bundle id and check ids
func (c *Check) CheckMeta() (*Meta, error) {
	if c.bundle != nil {
		return &Meta{
			BundleID: c.bundle.CID,
			CheckIDs: c.bundle.Checks,
		}, nil
	}
	return nil, errors.New("check not initialized")
}

// RefreshCheckConfig re-loads the check bundle using the API and reconfigures reverse (if needed)
func (c *Check) RefreshCheckConfig() error {
	c.Lock()
	defer c.Unlock()
	c.logger.Debug().Msg("refreshing check configuration using API")
	return c.setCheck()
}

// GetReverseConfig returns the reverse configuration to use for the broker
func (c *Check) GetReverseConfig() (*ReverseConfig, error) {
	c.Lock()
	defer c.Unlock()

	if c.revConfig == nil {
		return nil, errors.New("invalid reverse configuration")
	}
	return c.revConfig, nil
}

// EnableNewMetrics updates the check bundle enabling any new metrics
func (c *Check) EnableNewMetrics(m *cgm.Metrics) error {
	c.Lock()
	defer c.Unlock()

	if !c.manage {
		return nil
	}

	if !c.metricStateUpdate {
		// let first submission of metrics go through if no state file
		// use case where agent is replacing an existing nad install (check already exists)
		if c.metricStates == nil {
			c.logger.Debug().Msg("no existing metric states, triggering load")
			c.metricStateUpdate = true
			return nil
		}

		if time.Since(c.lastRefresh) > c.refreshTTL {
			c.logger.Debug().Dur("since_last", time.Since(c.lastRefresh)).Dur("ttl", c.refreshTTL).Msg("TTL triggering metric state refresh")
			c.metricStateUpdate = true
		}
	}

	if c.metricStateUpdate {
		err := c.setMetricStates(nil)
		if err != nil {
			return errors.Wrap(err, "updating metric states")
		}
	}

	c.logger.Debug().Msg("scanning for new metrics")

	newMetrics := map[string]api.CheckBundleMetric{}

	for mn, mv := range *m {
		if _, known := (*c.metricStates)[mn]; !known {
			newMetrics[mn] = c.configMetric(mn, mv)
			c.logger.Debug().Interface("metric", newMetrics[mn]).Interface("mv", mv).Msg("found new metric")
		}
	}

	if len(newMetrics) > 0 {
		if err := c.updateCheckBundleMetrics(&newMetrics); err != nil {
			c.logger.Error().Err(err).Msg("adding mew metrics to check bundle")
		}
	}

	return nil
}
