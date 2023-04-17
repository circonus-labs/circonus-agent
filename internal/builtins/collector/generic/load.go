// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/v3/load"
)

// Load metrics.
type Load struct {
	gencommon
}

// loadOptions defines what elements can be overridden in a config file.
type loadOptions struct {
	// common
	ID     string `json:"id" toml:"id" yaml:"id"`
	RunTTL string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewLoadCollector creates new psutils collector.
func NewLoadCollector(cfgBaseName string, parentLogger zerolog.Logger) (collector.Collector, error) {
	c := Load{}
	c.id = NameLoad
	c.pkgID = PackageName + "." + c.id
	c.logger = parentLogger.With().Str("pkg", PackageName).Str("id", c.id).Logger()
	c.baseTags = tags.FromList(tags.GetBaseTags())

	var opts loadOptions
	err := config.LoadConfigFile(cfgBaseName, &opts)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, fmt.Errorf("%s config: %w", c.pkgID, err)
	}

	c.logger.Debug().Interface("config", opts).Msg("loaded config")

	if opts.ID != "" {
		c.id = opts.ID
	}

	if opts.RunTTL != "" {
		dur, err := time.ParseDuration(opts.RunTTL)
		if err != nil {
			return nil, fmt.Errorf("%s parsing run_ttl: %w", c.pkgID, err)
		}
		c.runTTL = dur
	}

	return &c, nil
}

// Collect load metrics.
func (c *Load) Collect(ctx context.Context) error {
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

	tagUnitsProcesses := tags.Tag{Category: "units", Value: "processes"}

	metrics := cgm.Metrics{}
	loadavg, err := load.Avg()
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting load metrics")
	} else {
		tagList := tags.Tags{tagUnitsProcesses}
		if !math.IsNaN(loadavg.Load1) {
			_ = c.addMetric(&metrics, "load_1min", "n", loadavg.Load1, tagList)
		}
		if !math.IsNaN(loadavg.Load5) {
			_ = c.addMetric(&metrics, "load_5min", "n", loadavg.Load5, tagList)
		}
		if !math.IsNaN(loadavg.Load15) {
			_ = c.addMetric(&metrics, "load_15min", "n", loadavg.Load15, tagList)
		}
	}

	misc, err := load.MiscWithContext(ctx)
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting misc load metrics")
		c.setStatus(metrics, nil)
		return nil
	}

	{ // units:processes
		tagList := tags.Tags{tagUnitsProcesses}
		// _ = c.addMetric(&metrics, "created", "i", misc.ProcsCreated, tagList)
		_ = c.addMetric(&metrics, "running", "i", misc.ProcsRunning, tagList)
		_ = c.addMetric(&metrics, "blocked", "i", misc.ProcsBlocked, tagList)
		_ = c.addMetric(&metrics, "total", "i", misc.ProcsTotal, tagList)
	}
	{ // units:switches
		tagList := tags.Tags{tags.Tag{Category: "units", Value: "switches"}}
		_ = c.addMetric(&metrics, "ctxt", "i", misc.Ctxt, tagList)
	}

	c.setStatus(metrics, nil)
	return nil
}
