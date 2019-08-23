// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"crypto/tls"
	"net"
	"net/url"
	"path/filepath"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/go-apiclient"
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
	checkConfig           *apiclient.Check
	bundle                *apiclient.CheckBundle
	broker                *apiclient.Broker
	client                API
	lastRefresh           time.Time
	logger                zerolog.Logger
	manage                bool
	metricStates          *metricStates
	metricStateUpdate     bool
	refreshTTL            time.Duration
	reverse               bool
	revConfigs            *ReverseConfigs
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
	CN         string
	BrokerAddr *net.TCPAddr
	BrokerID   string
	ReverseURL *url.URL
	TLSConfig  *tls.Config
}

type ReverseConfigs map[string]ReverseConfig

const (
	StatusActive = "active"
)

type BundleNotActiveError struct {
	Err      string
	Checks   string
	BundleID string
	Status   string
}

func (e *BundleNotActiveError) Error() string {
	if e == nil {
		return "<nil>"
	}
	s := e.Err
	if e.BundleID != "" {
		s = s + "Bundle: " + e.BundleID + " "
	}
	if e.Checks != "" {
		s = s + "Check(s): " + e.Checks + " "
	}
	if e.Status != "" {
		s = s + "(" + e.Status + ")"
	}
	return s
}

type NoOwnerFoundError struct {
	Err      string
	BundleID string
}

func (e *NoOwnerFoundError) Error() string {
	if e == nil {
		return "<nil>"
	}
	s := e.Err
	if e.BundleID != "" {
		s = s + "Bundle: " + e.BundleID + " "
	}
	return s
}

// logshim is used to satisfy apiclient Logger interface (avoiding ptr receiver issue)
type logshim struct {
	logh zerolog.Logger
}

func (l logshim) Printf(fmt string, v ...interface{}) {
	l.logh.Printf(fmt, v...)
}

// New returns a new check instance
func New(apiClient API) (*Check, error) {
	// NOTE: TBD, make broker max retries and response time configurable
	c := Check{
		brokerMaxResponseTime: 500 * time.Millisecond,
		brokerMaxRetries:      5,
		bundle:                nil,
		broker:                nil,
		logger:                log.With().Str("pkg", "check").Logger(),
		manage:                false,
		metricStateUpdate:     false,
		refreshTTL:            time.Duration(0),
		reverse:               false,
		statePath:             viper.GetString(config.KeyCheckMetricStateDir),
		statusActiveBroker:    StatusActive,
		statusActiveMetric:    StatusActive,
	}

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
		cfg := &apiclient.Config{
			Debug:    viper.GetBool(config.KeyDebugAPI),
			Log:      logshim{logh: c.logger.With().Str("pkg", "circ.api").Logger()},
			TokenApp: viper.GetString(config.KeyAPITokenApp),
			TokenKey: viper.GetString(config.KeyAPITokenKey),
			URL:      viper.GetString(config.KeyAPIURL),
		}
		client, err := apiclient.New(cfg)
		if err != nil {
			return nil, errors.Wrap(err, "creating circonus api client")
		}
		apiClient = client
	}

	c.client = apiClient

	if err := c.initCheckBundle(cid, isCreate); err != nil {
		return nil, errors.Wrap(err, "initializing check")
	}

	c.logger.Debug().Interface("check_config", c.bundle).Msg("using check bundle config")

	// ensure a) the global check bundle id is set and b) it is set correctly
	// to the check bundle actually being used - need to do this even if the
	// check was created initially since user 'nobody' cannot create or update
	// the configuration (if one was used)
	viper.Set(config.KeyCheckBundleID, c.bundle.CID)

	if isReverse {
		err := c.setReverseConfigs()
		if err != nil {
			return nil, errors.Wrap(err, "setting up reverse configuration")
		}
		c.reverse = true
	}

	if len(c.bundle.MetricFilters) > 0 {
		c.logger.Debug().Msg("setting managed off, check has metric_filters")
		c.manage = false
		return &c, nil
	}

	if !isManaged {
		c.manage = false
		return &c, nil
	}

	c.stateFile = filepath.Join(c.statePath, "metrics.json")

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
		c.manage = false
		return &c, nil
	}

	err := c.setMetricStates(&c.bundle.Metrics)
	if err != nil {
		return nil, errors.Wrap(err, "setting metric states")
	}
	c.bundle.Metrics = []apiclient.CheckBundleMetric{} // save a little memory (or a lot depending on how many metrics are being managed...)

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
	c.Lock()
	defer c.Unlock()

	if c.bundle != nil {
		return &Meta{
			BundleID: c.bundle.CID,
			CheckIDs: c.bundle.Checks,
		}, nil
	}
	return nil, errors.New("check not initialized")
}

// CheckPeriod returns check bundle period (intetrval between when broker should make request)
func (c *Check) CheckPeriod() (uint, error) {
	c.Lock()
	defer c.Unlock()

	if c.bundle != nil {
		return c.bundle.Period, nil
	}
	return 0, errors.New("check not initialized")
}

// GetReverseConfigs returns the reverse connection configuration(s) to use for the check
func (c *Check) GetReverseConfigs() (*ReverseConfigs, error) {
	c.Lock()
	defer c.Unlock()

	if c.revConfigs == nil {
		return nil, errors.New("invalid reverse configuration")
	}
	return c.revConfigs, nil
}

// RefreshCheckConfig re-loads the check bundle using the API and reconfigures reverse (if needed)
func (c *Check) RefreshCheckConfig() error {
	c.Lock()
	defer c.Unlock()

	c.logger.Debug().Msg("refreshing check configuration using API")

	b, err := c.fetchCheckBundle(viper.GetString(config.KeyCheckBundleID))
	if err != nil {
		return errors.Wrap(err, "refresh check, fetching check")
	}

	c.bundle = b

	if c.manage {
		c.logger.Debug().Msg("setting metric states")
		err := c.setMetricStates(&c.bundle.Metrics)
		if err != nil {
			return errors.Wrap(err, "setting metric states")
		}
	}
	c.bundle.Metrics = []apiclient.CheckBundleMetric{} // save a little memory (or a lot depending on how many metrics are being managed...)

	if err := c.setReverseConfigs(); err != nil {
		return errors.Wrap(err, "refresh check, setting reverse config")
	}

	return nil
}

//
// NOTE: manually managing metrics is deprecated, allow/deny filters should
//       be used going forward. Methods related to metric management will
//       be removed in the future.
//

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

	newMetrics := map[string]apiclient.CheckBundleMetric{}

	for mn, mv := range *m {
		if _, known := (*c.metricStates)[mn]; !known {
			newMetrics[mn] = c.configMetric(mn, mv)
			c.logger.Debug().Interface("metric", newMetrics[mn]).Interface("mv", mv).Msg("found new metric")
		}
	}

	if len(newMetrics) > 0 {
		if err := c.updateCheckBundleMetrics(&newMetrics); err != nil {
			c.logger.Error().Err(err).Msg("adding new metrics to check bundle")
		}
	}

	return nil
}
