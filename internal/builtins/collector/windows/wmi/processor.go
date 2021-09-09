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
	"runtime"
	"strconv"
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

// Win32_PerfFormattedData_PerfOS_Processor defines the metrics to collect.
type Win32_PerfFormattedData_PerfOS_Processor struct { //nolint: golint
	Name                  string
	C1TransitionsPersec   uint64
	C2TransitionsPersec   uint64
	C3TransitionsPersec   uint64
	DPCsQueuedPersec      uint32
	InterruptsPersec      uint32
	PercentC1Time         uint64
	PercentC2Time         uint64
	PercentC3Time         uint64
	PercentDPCTime        uint64
	PercentIdleTime       uint64
	PercentInterruptTime  uint64
	PercentPrivilegedTime uint64
	PercentProcessorTime  uint64
	PercentUserTime       uint64
}

// Processor metrics from the Windows Management Interface (wmi).
type Processor struct {
	wmicommon
	numCPU        float64
	reportAllCPUs bool // may be overridden in config file
}

// processorOptions defines what elements can be overridden in a config file.
type processorOptions struct {
	ID              string `json:"id" toml:"id" yaml:"id"`
	AllCPU          string `json:"report_all_cpus" toml:"report_all_cpus" yaml:"report_all_cpus"`
	MetricNameRegex string `json:"metric_name_regex" toml:"metric_name_regex" yaml:"metric_name_regex"`
	MetricNameChar  string `json:"metric_name_char" toml:"metric_name_char" yaml:"metric_name_char"`
	RunTTL          string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewProcessorCollector creates new wmi collector.
func NewProcessorCollector(cfgBaseName string) (collector.Collector, error) {
	c := Processor{}
	c.id = "processor"
	c.pkgID = pkgName + "." + c.id
	c.logger = log.With().Str("pkg", pkgName).Str("id", c.id).Logger()
	c.metricNameChar = defaultMetricChar
	c.metricNameRegex = defaultMetricNameRegex
	c.baseTags = tags.FromList(tags.GetBaseTags())

	c.numCPU = float64(runtime.NumCPU())
	c.reportAllCPUs = true

	if cfgBaseName == "" {
		return &c, nil
	}

	var cfg processorOptions
	err := config.LoadConfigFile(cfgBaseName, &cfg)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, fmt.Errorf("%s config: %w", c.pkgID, err)
	}

	c.logger.Debug().Interface("config", cfg).Msg("loaded config")

	if cfg.AllCPU != "" {
		rpt, err := strconv.ParseBool(cfg.AllCPU)
		if err != nil {
			return nil, fmt.Errorf("%s parsing report_all_cpus: %w", c.pkgID, err)
		}
		c.reportAllCPUs = rpt
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
func (c *Processor) Collect(ctx context.Context) error {
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

	var dst []Win32_PerfFormattedData_PerfOS_Processor
	qry := wmi.CreateQuery(dst, "")
	if err := wmi.Query(qry, &dst); err != nil {
		c.logger.Error().Err(err).Str("query", qry).Msg("wmi query error")
		c.setStatus(metrics, err)
		return fmt.Errorf("wmi %s query: %w", c.pkgID, err)
	}

	metricType := "L"
	tagUnitsPercent := cgm.Tag{Category: "units", Value: "percent"}
	for _, item := range dst {
		cpuID := c.cleanName(item.Name)

		metricSuffix := ""
		if strings.Contains(item.Name, totalName) {
			cpuID = "all"
			metricSuffix = totalName
		} else if !c.reportAllCPUs {
			continue
		}

		cpuTag := cgm.Tag{Category: "cpu-id", Value: cpuID}

		_ = c.addMetric(&metrics, "", "PercentC1Time"+metricSuffix, metricType, item.PercentC1Time, cgm.Tags{cpuTag, tagUnitsPercent})
		_ = c.addMetric(&metrics, "", "PercentC2Time"+metricSuffix, metricType, item.PercentC2Time, cgm.Tags{cpuTag, tagUnitsPercent})
		_ = c.addMetric(&metrics, "", "PercentC3Time"+metricSuffix, metricType, item.PercentC3Time, cgm.Tags{cpuTag, tagUnitsPercent})
		_ = c.addMetric(&metrics, "", "PercentIdleTime"+metricSuffix, metricType, item.PercentIdleTime, cgm.Tags{cpuTag, tagUnitsPercent})
		_ = c.addMetric(&metrics, "", "PercentInterruptTime"+metricSuffix, metricType, item.PercentInterruptTime, cgm.Tags{cpuTag, tagUnitsPercent})
		_ = c.addMetric(&metrics, "", "PercentDPCTime"+metricSuffix, metricType, item.PercentDPCTime, cgm.Tags{cpuTag, tagUnitsPercent})
		_ = c.addMetric(&metrics, "", "PercentPrivilegedTime"+metricSuffix, metricType, item.PercentPrivilegedTime, cgm.Tags{cpuTag, tagUnitsPercent})
		_ = c.addMetric(&metrics, "", "PercentUserTime"+metricSuffix, metricType, item.PercentUserTime, cgm.Tags{cpuTag, tagUnitsPercent})
		_ = c.addMetric(&metrics, "", "PercentProcessorTime"+metricSuffix, metricType, item.PercentProcessorTime, cgm.Tags{cpuTag, tagUnitsPercent})
		_ = c.addMetric(&metrics, "", "C1TransitionsPersec"+metricSuffix, metricType, item.C1TransitionsPersec, cgm.Tags{cpuTag})
		_ = c.addMetric(&metrics, "", "C2TransitionsPersec"+metricSuffix, metricType, item.C2TransitionsPersec, cgm.Tags{cpuTag})
		_ = c.addMetric(&metrics, "", "C3TransitionsPersec"+metricSuffix, metricType, item.C3TransitionsPersec, cgm.Tags{cpuTag})
		_ = c.addMetric(&metrics, "", "InterruptsPersec"+metricSuffix, metricType, item.InterruptsPersec, cgm.Tags{cpuTag})
		_ = c.addMetric(&metrics, "", "DPCsQueuedPersec"+metricSuffix, metricType, item.DPCsQueuedPersec, cgm.Tags{cpuTag})
	}

	c.setStatus(metrics, nil)
	return nil
}
