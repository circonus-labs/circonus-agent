// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

//go:build windows
// +build windows

package wmi

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	// "github.com/StackExchange/wmi".
	"github.com/bi-zone/wmi"
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/rs/zerolog/log"
)

// Win32_PerfFormattedData_PerfOS_Objects defines the metrics to collect.
type Win32_PerfFormattedData_PerfOS_Objects struct { //nolint: revive
	Events     uint32
	Mutexes    uint32
	Processes  uint32
	Sections   uint32
	Semaphores uint32
	Threads    uint32
}

// Objects metrics from the Windows Management Interface (wmi).
type Objects struct {
	wmicommon
}

// objectsOptions defines what elements can be overridden in a config file.
type objectsOptions struct {
	ID              string `json:"id" toml:"id" yaml:"id"`
	MetricNameRegex string `json:"metric_name_regex" toml:"metric_name_regex" yaml:"metric_name_regex"`
	MetricNameChar  string `json:"metric_name_char" toml:"metric_name_char" yaml:"metric_name_char"`
	RunTTL          string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewObjectsCollector creates new wmi collector.
func NewObjectsCollector(cfgBaseName string) (collector.Collector, error) {
	c := Objects{}
	c.id = "objects"
	c.pkgID = pkgName + "." + c.id
	c.logger = log.With().Str("pkg", pkgName).Str("id", c.id).Logger()
	c.metricNameChar = defaultMetricChar
	c.metricNameRegex = defaultMetricNameRegex
	c.baseTags = tags.FromList(tags.GetBaseTags())

	if cfgBaseName == "" {
		return &c, nil
	}

	var cfg objectsOptions
	err := config.LoadConfigFile(cfgBaseName, &cfg)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Debug().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, fmt.Errorf("%s config: %w", c.pkgID, err)
	}

	c.logger.Debug().Interface("config", cfg).Msg("loaded config")

	if cfg.ID != "" {
		c.id = cfg.ID
	}

	if cfg.MetricNameRegex != "" {
		rx, err := regexp.Compile(cfg.MetricNameRegex)
		if err != nil {
			return nil, fmt.Errorf("%s compile metric_name_regex: %w", c.pkgID, err)
		}
		c.metricNameRegex = rx
	}

	if cfg.MetricNameChar != "" {
		c.metricNameChar = cfg.MetricNameChar
	}

	if cfg.RunTTL != "" {
		dur, err := time.ParseDuration(cfg.RunTTL)
		if err != nil {
			return nil, fmt.Errorf("%s parsing run_ttl: %w", c.pkgID, err)
		}
		c.runTTL = dur
	}

	return &c, nil
}

// Collect metrics from the wmi resource.
func (c *Objects) Collect(ctx context.Context) error {
	metrics := cgm.Metrics{}

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

	var dst []Win32_PerfFormattedData_PerfOS_Objects
	qry := wmi.CreateQuery(dst, "")
	if err := wmi.Query(qry, &dst); err != nil {
		c.logger.Error().Err(err).Str("query", qry).Msg("wmi query error")
		c.setStatus(metrics, err)
		return fmt.Errorf("wmi %s query: %w", c.pkgID, err)
	}

	metricType := "I"
	for _, item := range dst {
		if done(ctx) {
			return fmt.Errorf("context: %w", ctx.Err())
		}

		_ = c.addMetric(&metrics, "", "Events", metricType, item.Events, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "Mutexes", metricType, item.Mutexes, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "Processes", metricType, item.Processes, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "Sections", metricType, item.Sections, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "Semaphores", metricType, item.Semaphores, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "Threads", metricType, item.Threads, cgm.Tags{})
	}

	c.setStatus(metrics, nil)
	return nil
}
