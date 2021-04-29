// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

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

// Win32_PerfFormattedData_PerfOS_PagingFile defines the metrics to collect.
type Win32_PerfFormattedData_PerfOS_PagingFile struct { //nolint: golint
	Name         string
	PercentUsage uint32
}

// PagingFile metrics from the Windows Management Interface (wmi).
type PagingFile struct {
	include *regexp.Regexp
	exclude *regexp.Regexp
	wmicommon
}

// pagingFileOptions defines what elements can be overridden in a config file.
type pagingFileOptions struct {
	ID              string `json:"id" toml:"id" yaml:"id"`
	IncludeRegex    string `json:"include_regex" toml:"include_regex" yaml:"include_regex"`
	ExcludeRegex    string `json:"exclude_regex" toml:"exclude_regex" yaml:"exclude_regex"`
	MetricNameRegex string `json:"metric_name_regex" toml:"metric_name_regex" yaml:"metric_name_regex"`
	MetricNameChar  string `json:"metric_name_char" toml:"metric_name_char" yaml:"metric_name_char"`
	RunTTL          string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewPagingFileCollector creates new wmi collector.
func NewPagingFileCollector(cfgBaseName string) (collector.Collector, error) {
	c := PagingFile{}
	c.id = "paging_file"
	c.pkgID = pkgName + "." + c.id
	c.logger = log.With().Str("pkg", pkgName).Str("id", c.id).Logger()
	c.metricNameChar = defaultMetricChar
	c.metricNameRegex = defaultMetricNameRegex
	c.baseTags = tags.FromList(tags.GetBaseTags())

	c.include = defaultIncludeRegex
	c.exclude = defaultExcludeRegex

	if cfgBaseName == "" {
		return &c, nil
	}

	var cfg pagingFileOptions
	err := config.LoadConfigFile(cfgBaseName, &cfg)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Debug().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, fmt.Errorf("%s config: %w", c.pkgID, err)
	}

	c.logger.Debug().Interface("config", cfg).Msg("loaded config")

	// include regex
	if cfg.IncludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, cfg.IncludeRegex))
		if err != nil {
			return nil, fmt.Errorf("%s compiling include regex: %w", c.pkgID, err)
		}
		c.include = rx
	}

	// exclude regex
	if cfg.ExcludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, cfg.ExcludeRegex))
		if err != nil {
			return nil, fmt.Errorf("%s compiling exclude regex: %w", c.pkgID, err)
		}
		c.exclude = rx
	}

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
func (c *PagingFile) Collect(ctx context.Context) error {
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

	var dst []Win32_PerfFormattedData_PerfOS_PagingFile
	qry := wmi.CreateQuery(dst, "")
	if err := wmi.Query(qry, &dst); err != nil {
		c.logger.Error().Err(err).Str("query", qry).Msg("wmi query error")
		c.setStatus(metrics, err)
		return fmt.Errorf("wmi %s query: %w", c.pkgID, err)
	}

	metricType := "I"
	tagUnitsPercent := cgm.Tag{Category: "units", Value: "percent"}
	for _, item := range dst {
		itemName := c.cleanName(item.Name)
		if c.exclude.MatchString(itemName) || !c.include.MatchString(itemName) {
			continue
		}

		metricSuffix := ""
		if strings.Contains(item.Name, totalName) {
			itemName = "all"
			metricSuffix = totalName
		}

		fileTag := cgm.Tag{Category: "paging-file", Value: itemName}

		_ = c.addMetric(&metrics, "", "PercentUsage"+metricSuffix, metricType, item.PercentUsage, cgm.Tags{fileTag, tagUnitsPercent})
	}

	c.setStatus(metrics, nil)
	return nil
}
