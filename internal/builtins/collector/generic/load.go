// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/load"
)

// Load metrics
type Load struct {
	gencommon
}

// loadOptions defines what elements can be overridden in a config file
type loadOptions struct {
	// common
	ID     string `json:"id" toml:"id" yaml:"id"`
	RunTTL string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewLoadCollector creates new psutils collector
func NewLoadCollector(cfgBaseName string, parentLogger zerolog.Logger) (collector.Collector, error) {
	c := Load{}
	c.id = NameLoad
	c.pkgID = PackageName + "." + c.id
	c.logger = log.With().Str("pkg", PackageName).Str("id", c.id).Logger()
	c.baseTags = tags.FromList(tags.GetBaseTags())

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

	moduleTags := tags.Tags{
		tags.Tag{Category: "module", Value: c.id},
	}

	metrics := cgm.Metrics{}
	loadavg, err := load.Avg()
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting load metrics")
	} else {
		_ = c.addMetric(&metrics, "1min", "n", loadavg.Load1, moduleTags)
		_ = c.addMetric(&metrics, "5min", "n", loadavg.Load5, moduleTags)
		_ = c.addMetric(&metrics, "15min", "n", loadavg.Load15, moduleTags)
	}

	misc, err := load.Misc()
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting misc load metrics")
	} else {
		_ = c.addMetric(&metrics, "procs_running", "i", misc.ProcsRunning, moduleTags)
		_ = c.addMetric(&metrics, "procs_blocked", "i", misc.ProcsBlocked, moduleTags)
		_ = c.addMetric(&metrics, "ctxt", "i", misc.Ctxt, moduleTags)
	}

	c.setStatus(metrics, nil)
	return nil
}
