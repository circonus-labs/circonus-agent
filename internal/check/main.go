// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"fmt"
	stdlog "log"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-gometrics/api"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// New returns a new check instance
func New(apiClient API) (*Check, error) {
	c := Check{
		manage:       false,
		bundle:       nil,
		checkMetrics: make(map[string]api.CheckBundleMetric),
		knownMetrics: make(map[string]string),
		refreshTTL:   time.Duration(0),
		logger:       log.With().Str("pkg", "check").Logger(),
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

	if apiClient != nil {
		c.client = apiClient
	} else {
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

		c.client = client
	}

	if err := c.setCheck(); err != nil {
		return nil, errors.Wrap(err, "unable to configure check")
	}

	// ensure a) the global check bundle id is set and b) it is set correctly to the
	// check bundle actually being used - need to do this even if the check was
	// created initially since user 'nobody' cannot create or update the configuration
	viper.Set(config.KeyCheckBundleID, c.bundle.CID)

	if isManaged {
		// refresh ttl
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
	}

	return &c, nil
}

// RefreshCheckConfig re-loads the check bundle using the API and reconfigures reverse (if needed)
func (c *Check) RefreshCheckConfig() error {
	c.Lock()
	defer c.Unlock()
	return c.setCheck()
}

// GetReverseConfig returns the reverse configuration to use for the broker
func (c *Check) GetReverseConfig() (ReverseConfig, error) {
	c.Lock()
	defer c.Unlock()

	if c.revConfig == nil {
		return ReverseConfig{}, errors.New("invalid reverse configuration")
	}
	return *c.revConfig, nil
}

// EnableNewMetrics updates the check bundle enabling any new metrics
func (c *Check) EnableNewMetrics(m *map[string]interface{}) error {
	c.Lock()
	defer c.Unlock()

	if !c.manage {
		return nil
	}

	// on first submission just collect all metric names which will be submitted
	// and treat them all as "available"
	if len(c.knownMetrics) == 0 {
		for mn := range *m {
			c.logger.Debug().Str("name", mn).Msg("adding KNOWN metric")
			c.knownMetrics[mn] = "available"
		}
		return nil
	}

	// on second submission, fill in the checkMetrics list with metrics from
	// API (force pulled from broker)
	if len(c.checkMetrics) == 0 {
		c.logger.Debug().Msg("populating initial check metrics")
		metrics, err := c.getFullCheckMetrics()
		if err != nil {
			return errors.Wrap(err, "initial population of check metrics")
		}

		for _, metric := range metrics {
			if _, ok := c.checkMetrics[metric.Name]; !ok {
				c.logger.Debug().Str("name", metric.Name).Str("status", metric.Status).Msg("adding CHECK metric")
				c.checkMetrics[metric.Name] = metric
				if metric.Status == "active" {
					c.knownMetrics[metric.Name] = metric.Status
				}
			}
		}

		c.lastRefresh = time.Now()
		c.logger.Debug().Msg("initial population of check metrics done")
		return nil
	}

	if time.Since(c.lastRefresh) > c.refreshTTL {
		c.logger.Debug().Msg("refreshing check metrics")
		metrics, err := c.getFullCheckMetrics()
		if err != nil {
			return errors.Wrap(err, "refreshing check bundle metrics")
		}

		for _, metric := range metrics {
			if _, ok := c.checkMetrics[metric.Name]; !ok {
				c.logger.Debug().Str("name", metric.Name).Str("status", metric.Status).Msg("adding metric")
				c.checkMetrics[metric.Name] = metric
				if metric.Status == "active" {
					c.knownMetrics[metric.Name] = metric.Status
				}
			}
		}

		c.lastRefresh = time.Now()
		c.logger.Debug().Msg("refreshing check metrics done")
	}

	c.logger.Debug().Msg("scanning for new metrics")

	for mn, mv := range *m {
		if _, ok := c.checkMetrics[mn]; !ok {
			fmt.Printf("NOT FOUND %s = %#v\n", mn, mv)
		}
	}

	c.logger.Debug().Msg("enabling new metrics")

	// compare metric states
	// add any new metrics to check bundle
	// update check bundle via api if needed

	return nil
}
