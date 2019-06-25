// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build windows

package wmi

import (
	"regexp"
	"strings"
	"time"

	"github.com/StackExchange/wmi"
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// Win32_PerfFormattedData_PerfOS_Cache defines the metrics to collect
// https://wutils.com/wmi/root/cimv2/win32_perfformatteddata_perfos_cache/
type Win32_PerfFormattedData_PerfOS_Cache struct {
	AsyncCopyReadsPersec         uint32
	AsyncDataMapsPersec          uint32
	AsyncFastReadsPersec         uint32
	AsyncMDLReadsPersec          uint32
	AsyncPinReadsPersec          uint32
	CopyReadHitsPercent          uint32
	CopyReadsPersec              uint32
	DataFlushesPersec            uint32
	DataFlushPagesPersec         uint32
	DataMapHitsPercent           uint32
	DataMapPinsPersec            uint32
	DataMapsPersec               uint32
	DirtyPages                   uint64
	DirtyPageThreshold           uint64
	FastReadNotPossiblesPersec   uint32
	FastReadResourceMissesPersec uint32
	FastReadsPersec              uint32
	LazyWriteFlushesPersec       uint32
	LazyWritePagesPersec         uint32
	MDLReadHitsPercent           uint32
	MDLReadsPersec               uint32
	PinReadHitsPercent           uint32
	PinReadsPersec               uint32
	ReadAheadsPersec             uint32
	SyncCopyReadsPersec          uint32
	SyncDataMapsPersec           uint32
	SyncFastReadsPersec          uint32
	SyncMDLReadsPersec           uint32
	SyncPinReadsPersec           uint32
}

// Cache metrics from the Windows Management Interface (wmi)
type Cache struct {
	wmicommon
}

// cacheOptions defines what elements can be overridden in a config file
type cacheOptions struct {
	ID              string `json:"id" toml:"id" yaml:"id"`
	MetricNameRegex string `json:"metric_name_regex" toml:"metric_name_regex" yaml:"metric_name_regex"`
	MetricNameChar  string `json:"metric_name_char" toml:"metric_name_char" yaml:"metric_name_char"`
	RunTTL          string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewCacheCollector creates new wmi collector
func NewCacheCollector(cfgBaseName string) (collector.Collector, error) {
	c := Cache{}
	c.id = "cache"
	c.pkgID = pkgName + "." + c.id
	c.logger = log.With().Str("pkg", pkgName).Str("id", c.id).Logger()
	c.metricNameChar = defaultMetricChar
	c.metricNameRegex = defaultMetricNameRegex
	c.baseTags = tags.FromList(tags.GetBaseTags())

	if cfgBaseName == "" {
		return &c, nil
	}

	var cfg cacheOptions
	err := config.LoadConfigFile(cfgBaseName, &cfg)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Debug().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, errors.Wrapf(err, "%s config", c.pkgID)
	}

	c.logger.Debug().Interface("config", cfg).Msg("loaded config")

	if cfg.ID != "" {
		c.id = cfg.ID
	}

	if cfg.MetricNameRegex != "" {
		rx, err := regexp.Compile(cfg.MetricNameRegex)
		if err != nil {
			return nil, errors.Wrapf(err, "%s compile metric_name_regex", c.pkgID)
		}
		c.metricNameRegex = rx
	}

	if cfg.MetricNameChar != "" {
		c.metricNameChar = cfg.MetricNameChar
	}

	if cfg.RunTTL != "" {
		dur, err := time.ParseDuration(cfg.RunTTL)
		if err != nil {
			return nil, errors.Wrapf(err, "%s parsing run_ttl", c.pkgID)
		}
		c.runTTL = dur
	}

	return &c, nil
}

// Collect metrics from the wmi resource
func (c *Cache) Collect() error {
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

	metricType := "L"
	tagUnitsOperations := cgm.Tag{Category: "units", Value: "operations"}
	tagUnitsPercent := cgm.Tag{Category: "units", Value: "percent"}

	var dst []Win32_PerfFormattedData_PerfOS_Cache
	qry := wmi.CreateQuery(dst, "")
	if err := wmi.Query(qry, &dst); err != nil {
		c.logger.Error().Err(err).Str("query", qry).Msg("wmi query error")
		c.setStatus(metrics, err)
		return errors.Wrap(err, c.pkgID)
	}

	for _, item := range dst {
		_ = c.addMetric(&metrics, "", "AsyncCopyReadsPersec", metricType, item.AsyncCopyReadsPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "AsyncDataMapsPersec", metricType, item.AsyncDataMapsPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "AsyncFastReadsPersec", metricType, item.AsyncFastReadsPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "AsyncMDLReadsPersec", metricType, item.AsyncMDLReadsPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "AsyncPinReadsPersec", metricType, item.AsyncPinReadsPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "CopyReadHitsPercent", metricType, item.CopyReadHitsPercent, cgm.Tags{tagUnitsPercent})
		_ = c.addMetric(&metrics, "", "CopyReadsPersec", metricType, item.CopyReadsPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "DataFlushesPersec", metricType, item.DataFlushesPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "DataFlushPagesPersec", metricType, item.DataFlushPagesPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "DataMapHitsPercent", metricType, item.DataMapHitsPercent, cgm.Tags{tagUnitsPercent})
		_ = c.addMetric(&metrics, "", "DataMapPinsPersec", metricType, item.DataMapPinsPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "DataMapsPersec", metricType, item.DataMapsPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "DirtyPages", "I", item.DirtyPages, cgm.Tags{tagUnitsOperations})                 // uint64
		_ = c.addMetric(&metrics, "", "DirtyPageThreshold", "I", item.DirtyPageThreshold, cgm.Tags{tagUnitsOperations}) // uint64
		_ = c.addMetric(&metrics, "", "FastReadNotPossiblesPersec", metricType, item.FastReadNotPossiblesPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "FastReadResourceMissesPersec", metricType, item.FastReadResourceMissesPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "FastReadsPersec", metricType, item.FastReadsPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "LazyWriteFlushesPersec", metricType, item.LazyWriteFlushesPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "LazyWritePagesPersec", metricType, item.LazyWritePagesPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "MDLReadHitsPercent", metricType, item.MDLReadHitsPercent, cgm.Tags{tagUnitsPercent})
		_ = c.addMetric(&metrics, "", "MDLReadsPersec", metricType, item.MDLReadsPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "PinReadHitsPercent", metricType, item.PinReadHitsPercent, cgm.Tags{tagUnitsPercent})
		_ = c.addMetric(&metrics, "", "PinReadsPersec", metricType, item.PinReadsPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "ReadAheadsPersec", metricType, item.ReadAheadsPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "SyncCopyReadsPersec", metricType, item.SyncCopyReadsPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "SyncDataMapsPersec", metricType, item.SyncDataMapsPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "SyncFastReadsPersec", metricType, item.SyncFastReadsPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "SyncMDLReadsPersec", metricType, item.SyncMDLReadsPersec, cgm.Tags{tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "SyncPinReadsPersec", metricType, item.SyncPinReadsPersec, cgm.Tags{tagUnitsOperations})
	}

	c.setStatus(metrics, nil)
	return nil
}
