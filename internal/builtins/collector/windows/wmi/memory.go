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

// Win32_PerfFormattedData_PerfOS_Memory defines the metrics to collect.
type Win32_PerfFormattedData_PerfOS_Memory struct { //nolint: golint
	AvailableBytes                  uint64
	CacheBytes                      uint64
	CacheFaultsPersec               uint64
	CommittedBytes                  uint64
	DemandZeroFaultsPersec          uint64
	FreeAndZeroPageListBytes        uint64
	FreeSystemPageTableEntries      uint64
	ModifiedPageListBytes           uint64
	PageFaultsPersec                uint64
	PageReadsPersec                 uint64
	PagesInputPersec                uint64
	PagesOutputPersec               uint64
	PagesPersec                     uint64
	PageWritesPersec                uint64
	PercentCommittedBytesInUse      uint64
	PoolNonpagedAllocs              uint64
	PoolNonpagedBytes               uint64
	PoolPagedAllocs                 uint64
	PoolPagedBytes                  uint64
	PoolPagedResidentBytes          uint64
	StandbyCacheCoreBytes           uint64
	StandbyCacheNormalPriorityBytes uint64
	StandbyCacheReserveBytes        uint64
	SystemCacheResidentBytes        uint64
	SystemCodeResidentBytes         uint64
	SystemCodeTotalBytes            uint64
	SystemDriverTotalBytes          uint64
	TransitionFaultsPersec          uint64
	TransitionPagesRePurposedPersec uint64
	WriteCopiesPersec               uint64
}

// Memory metrics from the Windows Management Interface (wmi).
type Memory struct {
	wmicommon
}

// memoryOptions defines what elements can be overridden in a config file.
type memoryOptions struct {
	ID              string `json:"id" toml:"id" yaml:"id"`
	MetricNameRegex string `json:"metric_name_regex" toml:"metric_name_regex" yaml:"metric_name_regex"`
	MetricNameChar  string `json:"metric_name_char" toml:"metric_name_char" yaml:"metric_name_char"`
	RunTTL          string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewMemoryCollector creates new wmi collector.
func NewMemoryCollector(cfgBaseName string) (collector.Collector, error) {
	c := Memory{}
	c.id = "memory"
	c.pkgID = pkgName + "." + c.id
	c.logger = log.With().Str("pkg", pkgName).Str("id", c.id).Logger()
	c.metricNameChar = defaultMetricChar
	c.metricNameRegex = defaultMetricNameRegex
	c.baseTags = tags.FromList(tags.GetBaseTags())

	if cfgBaseName == "" {
		return &c, nil
	}

	var cfg memoryOptions
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
			return nil, fmt.Errorf("%s compile metric name rx: %w", c.pkgID, err)
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
func (c *Memory) Collect(ctx context.Context) error {
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

	var dst []Win32_PerfFormattedData_PerfOS_Memory
	qry := wmi.CreateQuery(dst, "")
	if err := wmi.Query(qry, &dst); err != nil {
		c.logger.Error().Err(err).Str("query", qry).Msg("wmi query error")
		c.setStatus(metrics, err)
		return fmt.Errorf("wmi %s query: %w", c.pkgID, err)
	}

	metricType := "L"
	tagUnitsBytes := cgm.Tag{Category: "units", Value: "bytes"}
	tagUnitsOperations := cgm.Tag{Category: "units", Value: "operations"}

	if len(dst) > 1 {
		c.logger.Warn().Int("len", len(dst)).Msg("memory metrics has more than one SET of enteries")
	}

	for _, item := range dst {
		_ = c.addMetric(&metrics, "", "AvailableBytes", metricType, item.AvailableBytes, cgm.Tags{tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "CacheBytes", metricType, item.CacheBytes, cgm.Tags{tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "CacheFaultsPersec", metricType, item.CacheFaultsPersec, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "CommittedBytes", metricType, item.CommittedBytes, cgm.Tags{tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "DemandZeroFaultsPersec", metricType, item.DemandZeroFaultsPersec, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "FreeAndZeroPageListBytes", metricType, item.FreeAndZeroPageListBytes, cgm.Tags{tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "FreeSystemPageTableEntries", metricType, item.FreeSystemPageTableEntries, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "ModifiedPageListBytes", metricType, item.ModifiedPageListBytes, cgm.Tags{tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "PageFaultsPersec", metricType, item.PageFaultsPersec, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "PageReadsPersec", metricType, item.PageReadsPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "PagesInputPersec", metricType, item.PagesInputPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "PagesOutputPersec", metricType, item.PagesOutputPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "PagesPersec", metricType, item.PagesPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "PageWritesPersec", metricType, item.PageWritesPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "PercentCommittedBytesInUse", metricType, item.PercentCommittedBytesInUse, cgm.Tags{tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "PoolNonpagedAllocs", metricType, item.PoolNonpagedAllocs, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "PoolNonpagedBytes", metricType, item.PoolNonpagedBytes, cgm.Tags{tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "PoolPagedAllocs", metricType, item.PoolPagedAllocs, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "PoolPagedBytes", metricType, item.PoolPagedBytes, cgm.Tags{tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "PoolPagedResidentBytes", metricType, item.PoolPagedResidentBytes, cgm.Tags{tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "StandbyCacheCoreBytes", metricType, item.StandbyCacheCoreBytes, cgm.Tags{tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "StandbyCacheNormalPriorityBytes", metricType, item.StandbyCacheNormalPriorityBytes, cgm.Tags{tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "StandbyCacheReserveBytes", metricType, item.StandbyCacheReserveBytes, cgm.Tags{tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "SystemCacheResidentBytes", metricType, item.SystemCacheResidentBytes, cgm.Tags{tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "SystemCodeResidentBytes", metricType, item.SystemCodeResidentBytes, cgm.Tags{tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "SystemCodeTotalBytes", metricType, item.SystemCodeTotalBytes, cgm.Tags{tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "SystemDriverTotalBytes", metricType, item.SystemDriverTotalBytes, cgm.Tags{tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "TransitionFaultsPersec", metricType, item.TransitionFaultsPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "TransitionPagesRePurposedPersec", metricType, item.TransitionPagesRePurposedPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "WriteCopiesPersec", metricType, item.WriteCopiesPersec, cgm.Tags{tagUnitsOperations})
	}

	c.setStatus(metrics, nil)
	return nil
}
