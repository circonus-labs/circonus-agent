// Copyright © 2020 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build windows

package wmi

import (
	"context"
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

//nolint:golint // ignore underscore in names, needed for wmi pkg
type Win32_PerfFormattedData_PerfOS_System struct {
	Caption                     string
	Description                 string
	Name                        string
	FileControlBytesPerSec      uint64
	FileReadBytesPerSec         uint64
	FileWriteBytesPerSec        uint64
	Frequency_Object            uint64
	Frequency_PerfTime          uint64
	Frequency_Sys100NS          uint64
	SystemUpTime                uint64
	Timestamp_Object            uint64
	Timestamp_PerfTime          uint64
	Timestamp_Sys100NS          uint64
	AlignmentFixupsPerSec       uint32
	ContextSwitchesPerSec       uint32
	ExceptionDispatchesPerSec   uint32
	FileControlOperationsPerSec uint32
	FileDataOperationsPerSec    uint32
	FileReadOperationsPerSec    uint32
	FileWriteOperationsPerSec   uint32
	FloatingEmulationsPerSec    uint32
	PercentRegistryQuotaInUse   uint32
	Processes                   uint32
	ProcessorQueueLength        uint32
	SystemCallsPerSec           uint32
	Threads                     uint32
}

// System metrics from the Windows Management Interface (wmi)
type System struct {
	wmicommon
}

// systemOptions defines what elements can be overridden in a config file
type systemOptions struct {
	ID              string `json:"id" toml:"id" yaml:"id"`
	MetricNameRegex string `json:"metric_name_regex" toml:"metric_name_regex" yaml:"metric_name_regex"`
	MetricNameChar  string `json:"metric_name_char" toml:"metric_name_char" yaml:"metric_name_char"`
	RunTTL          string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewSystemCollector creates new wmi collector
func NewSystemCollector(cfgBaseName string) (collector.Collector, error) {
	c := System{}
	c.id = "system"
	c.pkgID = pkgName + "." + c.id
	c.logger = log.With().Str("pkg", pkgName).Str("id", c.id).Logger()
	c.metricNameChar = defaultMetricChar
	c.metricNameRegex = defaultMetricNameRegex
	c.baseTags = tags.FromList(tags.GetBaseTags())

	if cfgBaseName == "" {
		return &c, nil
	}

	var cfg systemOptions
	err := config.LoadConfigFile(cfgBaseName, &cfg)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
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
func (c *System) Collect(ctx context.Context) error {
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

	var dst []Win32_PerfFormattedData_PerfOS_System
	qry := wmi.CreateQuery(dst, "")
	if err := wmi.Query(qry, &dst); err != nil {
		c.logger.Error().Err(err).Str("query", qry).Msg("wmi query error")
		c.setStatus(metrics, err)
		return errors.Wrap(err, c.pkgID)
	}

	metricType := "L"
	tagUnitsPercent := cgm.Tag{Category: "units", Value: "percent"}
	for _, item := range dst {
		_ = c.addMetric(&metrics, "", "AlignmentFixupsPerSec", metricType, item.AlignmentFixupsPerSec, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "ContextSwitchesPerSec", metricType, item.ContextSwitchesPerSec, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "ExceptionDispatchesPerSec", metricType, item.ExceptionDispatchesPerSec, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "FileControlBytesPerSec", metricType, item.FileControlBytesPerSec, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "FileControlOperationsPerSec", metricType, item.FileControlOperationsPerSec, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "FileDataOperationsPerSec", metricType, item.FileDataOperationsPerSec, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "FileReadBytesPerSec", metricType, item.FileReadBytesPerSec, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "FileReadOperationsPerSec", metricType, item.FileReadOperationsPerSec, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "FileWriteBytesPerSec", metricType, item.FileWriteBytesPerSec, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "FileWriteOperationsPerSec", metricType, item.FileWriteOperationsPerSec, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "FloatingEmulationsPerSec", metricType, item.FloatingEmulationsPerSec, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "PercentRegistryQuotaInUse", metricType, item.PercentRegistryQuotaInUse, cgm.Tags{tagUnitsPercent})
		_ = c.addMetric(&metrics, "", "Processes", metricType, item.Processes, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "ProcessorQueueLength", metricType, item.ProcessorQueueLength, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "SystemCallsPerSec", metricType, item.SystemCallsPerSec, cgm.Tags{})
		_ = c.addMetric(&metrics, "", "Threads", metricType, item.Threads, cgm.Tags{})
	}

	c.setStatus(metrics, nil)
	return nil
}
