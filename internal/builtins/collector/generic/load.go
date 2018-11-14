// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
	"regexp"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/load"
)

// Load metrics
type Load struct {
	common
}

// loadOptions defines what elements can be overridden in a config file
type loadOptions struct {
	// common
	ID                   string   `json:"id" toml:"id" yaml:"id"`
	MetricsEnabled       []string `json:"metrics_enabled" toml:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsDisabled      []string `json:"metrics_disabled" toml:"metrics_disabled" yaml:"metrics_disabled"`
	MetricsDefaultStatus string   `json:"metrics_default_status" toml:"metrics_default_status" toml:"metrics_default_status"`
	RunTTL               string   `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewLoadCollector creates new psutils collector
func NewLoadCollector(cfgBaseName string) (collector.Collector, error) {
	c := Load{}
	c.id = LOAD_NAME
	c.pkgID = LOG_PREFIX + c.id
	c.logger = log.With().Str("pkg", c.pkgID).Logger()
	c.metricStatus = map[string]bool{}
	c.metricDefaultActive = true

	var opts loadOptions
	err := config.LoadConfigFile(cfgBaseName, &opts)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, errors.Wrapf(err, "%s config", c.pkgID)
	}

	c.logger.Debug().Interface("config", opts).Msg("loaded config")

	if opts.ID != "" {
		c.id = opts.ID
	}

	if len(opts.MetricsEnabled) > 0 {
		for _, name := range opts.MetricsEnabled {
			c.metricStatus[name] = true
		}
	}
	if len(opts.MetricsDisabled) > 0 {
		for _, name := range opts.MetricsDisabled {
			c.metricStatus[name] = false
		}
	}

	if opts.MetricsDefaultStatus != "" {
		if ok, _ := regexp.MatchString(`^(enabled|disabled)$`, strings.ToLower(opts.MetricsDefaultStatus)); ok {
			c.metricDefaultActive = strings.ToLower(opts.MetricsDefaultStatus) == metricStatusEnabled
		} else {
			return nil, errors.Errorf("%s invalid metric default status (%s)", c.pkgID, opts.MetricsDefaultStatus)
		}
	}

	if opts.RunTTL != "" {
		dur, err := time.ParseDuration(opts.RunTTL)
		if err != nil {
			return nil, errors.Wrapf(err, "%s parsing run_ttl", c.pkgID)
		}
		c.runTTL = dur
	}

	return &c, nil
}

// Collect load metrics
func (c *Load) Collect() error {
	c.Lock()
	if c.runTTL > time.Duration(0) {
		if time.Since(c.lastEnd) < c.runTTL {
			c.logger.Warn().Msg(collector.ErrTTLNotExpired.Error())
			c.Unlock()
			return collector.ErrTTLNotExpired
		}
	}
	if c.running {
		c.logger.Warn().Msg(collector.ErrAlreadyRunning.Error())
		c.Unlock()
		return collector.ErrAlreadyRunning
	}

	c.running = true
	c.lastStart = time.Now()
	c.Unlock()

	metrics := cgm.Metrics{}

	loadavg, err := load.Avg()
	if err != nil {
		c.logger.Warn().Err(err).Str("id", c.id).Msg("collecting load metrics")
	} else {
		c.addMetric(&metrics, c.id, "1min", "n", loadavg.Load1)
		c.addMetric(&metrics, c.id, "5min", "n", loadavg.Load5)
		c.addMetric(&metrics, c.id, "15min", "n", loadavg.Load15)
	}

	misc, err := load.Misc()
	if err != nil {
		c.logger.Warn().Err(err).Str("id", c.id).Msg("collecting misc load metrics")
	} else {
		c.addMetric(&metrics, c.id, "procs_running", "i", misc.ProcsRunning)
		c.addMetric(&metrics, c.id, "procs_blocked", "i", misc.ProcsBlocked)
		c.addMetric(&metrics, c.id, "ctxt", "i", misc.Ctxt)
	}

	c.setStatus(metrics, nil)
	return nil
}
