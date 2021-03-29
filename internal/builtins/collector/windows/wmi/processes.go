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

	"github.com/StackExchange/wmi"
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// Win32_PerfFormattedData_PerfProc_Process defines the metrics to collect
// https://technet.microsoft.com/en-ca/aa394277(v=vs.71)
type Win32_PerfFormattedData_PerfProc_Process struct { //nolint: golint
	Name                    string
	ElapsedTime             uint64
	IODataBytesPersec       uint64
	IODataOperationsPersec  uint64
	IOOtherBytesPersec      uint64
	IOOtherOperationsPersec uint64
	IOReadBytesPersec       uint64
	IOReadOperationsPersec  uint64
	IOWriteBytesPersec      uint64
	IOWriteOperationsPersec uint64
	PageFileBytes           uint64
	PageFileBytesPeak       uint64
	PercentPrivilegedTime   uint64
	PercentProcessorTime    uint64
	PercentUserTime         uint64
	PrivateBytes            uint64
	VirtualBytes            uint64
	VirtualBytesPeak        uint64
	WorkingSet              uint64
	WorkingSetPeak          uint64
	WorkingSetPrivate       uint64
	CreatingProcessID       uint32
	HandleCount             uint32
	IDProcess               uint32
	PageFaultsPersec        uint32
	PoolNonpagedBytes       uint32
	PoolPagedBytes          uint32
	PriorityBase            uint32
	ThreadCount             uint32
}

// Processes metrics from the Windows Management Interface (wmi)
type Processes struct {
	include *regexp.Regexp
	exclude *regexp.Regexp
	wmicommon
}

// ProcessesOptions defines what elements can be overridden in a config file
type ProcessesOptions struct {
	ID              string `json:"id" toml:"id" yaml:"id"`
	IncludeRegex    string `json:"include_regex" toml:"include_regex" yaml:"include_regex"`
	ExcludeRegex    string `json:"exclude_regex" toml:"exclude_regex" yaml:"exclude_regex"`
	MetricNameRegex string `json:"metric_name_regex" toml:"metric_name_regex" yaml:"metric_name_regex"`
	MetricNameChar  string `json:"metric_name_char" toml:"metric_name_char" yaml:"metric_name_char"`
	RunTTL          string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewProcessesCollector creates new wmi collector
func NewProcessesCollector(cfgBaseName string) (collector.Collector, error) {
	c := Processes{}
	c.id = "processes"
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

	var cfg ProcessesOptions
	err := config.LoadConfigFile(cfgBaseName, &cfg)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Debug().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, errors.Wrapf(err, "%s config", c.pkgID)
	}

	c.logger.Debug().Interface("config", cfg).Msg("loaded config")

	// include regex
	if cfg.IncludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, cfg.IncludeRegex))
		if err != nil {
			return nil, errors.Wrapf(err, "%s compiling include regex", c.pkgID)
		}
		c.include = rx
	}

	// exclude regex
	if cfg.ExcludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, cfg.ExcludeRegex))
		if err != nil {
			return nil, errors.Wrapf(err, "%s compiling exclude regex", c.pkgID)
		}
		c.exclude = rx
	}

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
func (c *Processes) Collect(ctx context.Context) error {
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

	var dst []Win32_PerfFormattedData_PerfProc_Process
	qry := wmi.CreateQuery(dst, "")
	if err := wmi.Query(qry, &dst); err != nil {
		c.logger.Error().Err(err).Str("query", qry).Msg("wmi query error")
		c.setStatus(metrics, err)
		return errors.Wrap(err, c.pkgID)
	}

	metricTypeUint32 := "I"
	metricTypeUint64 := "L"
	tagUnitsSeconds := cgm.Tag{Category: "units", Value: "seconds"}
	tagUnitsBytes := cgm.Tag{Category: "units", Value: "bytes"}
	tagUnitsOperations := cgm.Tag{Category: "units", Value: "operations"}
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

		nameTag := cgm.Tag{Category: "process-name", Value: itemName}

		_ = c.addMetric(&metrics, "", "CreatingProcessID"+metricSuffix, metricTypeUint32, item.CreatingProcessID, cgm.Tags{nameTag})
		_ = c.addMetric(&metrics, "", "ElapsedTime"+metricSuffix, metricTypeUint64, item.ElapsedTime, cgm.Tags{nameTag, tagUnitsSeconds})
		_ = c.addMetric(&metrics, "", "HandleCount"+metricSuffix, metricTypeUint32, item.HandleCount, cgm.Tags{nameTag})
		_ = c.addMetric(&metrics, "", "IDProcess"+metricSuffix, metricTypeUint32, item.IDProcess, cgm.Tags{nameTag})
		_ = c.addMetric(&metrics, "", "IODataBytesPersec"+metricSuffix, metricTypeUint64, item.IODataBytesPersec, cgm.Tags{nameTag, tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "IODataOperationsPersec"+metricSuffix, metricTypeUint64, item.IODataOperationsPersec, cgm.Tags{nameTag, tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "IOOtherBytesPersec"+metricSuffix, metricTypeUint64, item.IOOtherBytesPersec, cgm.Tags{nameTag, tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "IOOtherOperationsPersec"+metricSuffix, metricTypeUint64, item.IOOtherOperationsPersec, cgm.Tags{nameTag, tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "IOReadBytesPersec"+metricSuffix, metricTypeUint64, item.IOReadBytesPersec, cgm.Tags{nameTag, tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "IOReadOperationsPersec"+metricSuffix, metricTypeUint64, item.IOReadOperationsPersec, cgm.Tags{nameTag, tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "IOWriteBytesPersec"+metricSuffix, metricTypeUint64, item.IOWriteBytesPersec, cgm.Tags{nameTag, tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "IOWriteOperationsPersec"+metricSuffix, metricTypeUint64, item.IOWriteOperationsPersec, cgm.Tags{nameTag, tagUnitsOperations})
		_ = c.addMetric(&metrics, "", "PageFaultsPersec"+metricSuffix, metricTypeUint32, item.PageFaultsPersec, cgm.Tags{nameTag})
		_ = c.addMetric(&metrics, "", "PageFileBytes"+metricSuffix, metricTypeUint64, item.PageFileBytes, cgm.Tags{nameTag, tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "PageFileBytesPeak"+metricSuffix, metricTypeUint64, item.PageFileBytesPeak, cgm.Tags{nameTag, tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "PercentPrivilegedTime"+metricSuffix, metricTypeUint64, item.PercentPrivilegedTime, cgm.Tags{nameTag, tagUnitsPercent})
		_ = c.addMetric(&metrics, "", "PercentProcessorTime"+metricSuffix, metricTypeUint64, item.PercentProcessorTime, cgm.Tags{nameTag, tagUnitsPercent})
		_ = c.addMetric(&metrics, "", "PercentUserTime"+metricSuffix, metricTypeUint64, item.PercentUserTime, cgm.Tags{nameTag, tagUnitsPercent})
		_ = c.addMetric(&metrics, "", "PoolNonpagedBytes"+metricSuffix, metricTypeUint32, item.PoolNonpagedBytes, cgm.Tags{nameTag, tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "PoolPagedBytes"+metricSuffix, metricTypeUint32, item.PoolPagedBytes, cgm.Tags{nameTag, tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "PriorityBase"+metricSuffix, metricTypeUint32, item.PriorityBase, cgm.Tags{nameTag})
		_ = c.addMetric(&metrics, "", "PrivateBytes"+metricSuffix, metricTypeUint64, item.PrivateBytes, cgm.Tags{nameTag, tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "ThreadCount"+metricSuffix, metricTypeUint32, item.ThreadCount, cgm.Tags{nameTag})
		_ = c.addMetric(&metrics, "", "VirtualBytes"+metricSuffix, metricTypeUint64, item.VirtualBytes, cgm.Tags{nameTag, tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "VirtualBytesPeak"+metricSuffix, metricTypeUint64, item.VirtualBytesPeak, cgm.Tags{nameTag, tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "WorkingSet"+metricSuffix, metricTypeUint64, item.WorkingSet, cgm.Tags{nameTag, tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "WorkingSetPeak"+metricSuffix, metricTypeUint64, item.WorkingSetPeak, cgm.Tags{nameTag, tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "WorkingSetPrivate"+metricSuffix, metricTypeUint64, item.WorkingSetPrivate, cgm.Tags{nameTag, tagUnitsBytes})
	}

	c.setStatus(metrics, nil)
	return nil
}
