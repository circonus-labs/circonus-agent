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

type genericDiskMetrics struct {
	Name                    string
	AvgDiskBytesPerRead     uint64
	AvgDiskBytesPerTransfer uint64
	AvgDiskBytesPerWrite    uint64
	AvgDiskQueueLength      uint64
	AvgDiskReadQueueLength  uint64
	AvgDiskWriteQueueLength uint64
	DiskBytesPersec         uint64
	DiskReadBytesPersec     uint64
	DiskWriteBytesPersec    uint64
	DiskWritesPersec        uint64
	PercentDiskReadTime     uint64
	PercentDiskTime         uint64
	PercentDiskWriteTime    uint64
	PercentIdleTime         uint64
	AvgDisksecPerRead       uint32
	AvgDisksecPerTransfer   uint32
	AvgDisksecPerWrite      uint32
	CurrentDiskQueueLength  uint32
	DiskReadsPersec         uint32
	DiskTransfersPersec     uint32
	FreeMegabytes           uint32
	PercentFreeSpace        uint32
	SplitIOPerSec           uint32
}

// Win32_PerfFormattedData_PerfDisk_LogicalDisk defines the metrics to collect
// https://technet.microsoft.com/en-ca/aa394261(v=vs.71)
type Win32_PerfFormattedData_PerfDisk_LogicalDisk struct { //nolint: revive
	Name                    string
	AvgDiskBytesPerRead     uint64
	AvgDiskBytesPerTransfer uint64
	AvgDiskBytesPerWrite    uint64
	AvgDiskQueueLength      uint64
	AvgDiskReadQueueLength  uint64
	AvgDiskWriteQueueLength uint64
	DiskBytesPersec         uint64
	DiskReadBytesPersec     uint64
	DiskWriteBytesPersec    uint64
	DiskWritesPersec        uint64
	PercentDiskReadTime     uint64
	PercentDiskTime         uint64
	PercentDiskWriteTime    uint64
	PercentIdleTime         uint64
	AvgDisksecPerRead       uint32
	AvgDisksecPerTransfer   uint32
	AvgDisksecPerWrite      uint32
	CurrentDiskQueueLength  uint32
	DiskReadsPersec         uint32
	DiskTransfersPersec     uint32
	FreeMegabytes           uint32
	PercentFreeSpace        uint32
	SplitIOPerSec           uint32
}

// Win32_PerfFormattedData_PerfDisk_PhysicalDisk defines the metrics to collect.
type Win32_PerfFormattedData_PerfDisk_PhysicalDisk struct { //nolint: revive
	Name                    string
	AvgDiskBytesPerRead     uint64
	AvgDiskBytesPerTransfer uint64
	AvgDiskBytesPerWrite    uint64
	AvgDiskQueueLength      uint64
	AvgDiskReadQueueLength  uint64
	AvgDiskWriteQueueLength uint64
	DiskBytesPersec         uint64
	DiskReadBytesPersec     uint64
	DiskWriteBytesPersec    uint64
	DiskWritesPersec        uint64
	PercentDiskReadTime     uint64
	PercentDiskTime         uint64
	PercentDiskWriteTime    uint64
	PercentIdleTime         uint64
	CurrentDiskQueueLength  uint32
	AvgDisksecPerRead       uint32
	AvgDisksecPerTransfer   uint32
	AvgDisksecPerWrite      uint32
	DiskReadsPersec         uint32
	DiskTransfersPersec     uint32
	SplitIOPerSec           uint32
}

// Disk metrics from the Windows Management Interface (wmi).
type Disk struct {
	include *regexp.Regexp
	exclude *regexp.Regexp
	wmicommon
	logical  bool
	physical bool
}

// diskOptions defines what elements can be overridden in a config file.
type diskOptions struct {
	ID              string `json:"id" toml:"id" yaml:"id"`
	IncludeLogical  string `json:"logical_disks" toml:"logical_disks" yaml:"logical_disks"`
	IncludePhysical string `json:"physical_disks" toml:"physical_disks" yaml:"physical_disks"`
	IncludeRegex    string `json:"include_regex" toml:"include_regex" yaml:"include_regex"`
	ExcludeRegex    string `json:"exclude_regex" toml:"exclude_regex" yaml:"exclude_regex"`
	MetricNameRegex string `json:"metric_name_regex" toml:"metric_name_regex" yaml:"metric_name_regex"`
	MetricNameChar  string `json:"metric_name_char" toml:"metric_name_char" yaml:"metric_name_char"`
	RunTTL          string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewDiskCollector creates new wmi collector.
func NewDiskCollector(cfgBaseName string) (collector.Collector, error) {
	c := Disk{}
	c.id = "disk"
	c.wmicommon.pkgID = pkgName + "." + c.id
	c.logger = log.With().Str("pkg", pkgName).Str("id", c.id).Logger()
	c.metricNameChar = defaultMetricChar
	c.metricNameRegex = defaultMetricNameRegex
	c.baseTags = tags.FromList(tags.GetBaseTags())

	c.logical = true
	c.physical = true
	c.include = defaultIncludeRegex
	c.exclude = defaultExcludeRegex

	if cfgBaseName == "" {
		return &c, nil
	}

	var cfg diskOptions
	err := config.LoadConfigFile(cfgBaseName, &cfg)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Debug().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, fmt.Errorf("%s config: %w", c.pkgID, err)
	}

	c.logger.Debug().Interface("config", cfg).Msg("loaded config")

	if cfg.IncludeLogical != "" {
		logical, err := strconv.ParseBool(cfg.IncludeLogical)
		if err != nil {
			return nil, fmt.Errorf("%s parsing disks: %w", c.pkgID, err)
		}
		c.logical = logical
	}

	if cfg.IncludePhysical != "" {
		physical, err := strconv.ParseBool(cfg.IncludePhysical)
		if err != nil {
			return nil, fmt.Errorf("%s parsing physical_disks: %w", c.pkgID, err)
		}
		c.physical = physical
	}

	// include regex
	if cfg.IncludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, cfg.IncludeRegex))
		if err != nil {
			return nil, fmt.Errorf("%s compile include rx: %w", c.pkgID, err)
		}
		c.include = rx
	}

	// exclude regex
	if cfg.ExcludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, cfg.ExcludeRegex))
		if err != nil {
			return nil, fmt.Errorf("%s compile exclude rx: %w", c.pkgID, err)
		}
		c.exclude = rx
	}

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
		c.wmicommon.runTTL = dur
	}

	return &c, nil
}

// Collect metrics from the wmi resource.
func (c *Disk) Collect(ctx context.Context) error {
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

	if c.logical {
		var dst []Win32_PerfFormattedData_PerfDisk_LogicalDisk
		qry := wmi.CreateQuery(dst, "")
		if err := wmi.Query(qry, &dst); err != nil {
			c.logger.Error().Err(err).Str("query", qry).Msg("wmi query error")
			c.setStatus(metrics, err)
			return fmt.Errorf("wmi %s query: %w", c.pkgID, err)
		}

		if len(dst) == 0 {
			c.logger.Debug().Msg("skipping logical disk metrics, no logical disks found")
		}

		for _, diskMetrics := range dst {
			if done(ctx) {
				return fmt.Errorf("context: %w", ctx.Err())
			}

			dm := diskMetrics
			_ = c.emitLogicalDiskMetrics(ctx, &metrics, &dm)
		}
	}

	if c.physical {
		var dst []Win32_PerfFormattedData_PerfDisk_PhysicalDisk
		qry := wmi.CreateQuery(dst, "")
		if err := wmi.Query(qry, &dst); err != nil {
			c.logger.Error().Err(err).Str("query", qry).Msg("wmi query error")
			c.setStatus(metrics, err)
			return fmt.Errorf("wmi %s query: %w", c.pkgID, err)
		}

		if len(dst) == 0 {
			c.logger.Debug().Msg("skipping physical disk metrics, no physical disks found")
		}

		for _, diskMetrics := range dst {
			if done(ctx) {
				return fmt.Errorf("context: %w", ctx.Err())
			}

			dm := diskMetrics
			_ = c.emitPhysicalDiskMetrics(ctx, &metrics, &dm)
		}
	}

	c.setStatus(metrics, nil)
	return nil
}

func (c *Disk) emitLogicalDiskMetrics(ctx context.Context, metrics *cgm.Metrics, diskMetrics *Win32_PerfFormattedData_PerfDisk_LogicalDisk) error {
	dm := genericDiskMetrics{
		Name:                    diskMetrics.Name,
		AvgDiskBytesPerRead:     diskMetrics.AvgDiskBytesPerRead,
		AvgDiskBytesPerTransfer: diskMetrics.AvgDiskBytesPerTransfer,
		AvgDiskBytesPerWrite:    diskMetrics.AvgDiskBytesPerWrite,
		AvgDiskQueueLength:      diskMetrics.AvgDiskQueueLength,
		AvgDiskReadQueueLength:  diskMetrics.AvgDiskReadQueueLength,
		AvgDisksecPerRead:       diskMetrics.AvgDisksecPerRead,
		AvgDisksecPerTransfer:   diskMetrics.AvgDisksecPerTransfer,
		AvgDisksecPerWrite:      diskMetrics.AvgDisksecPerWrite,
		AvgDiskWriteQueueLength: diskMetrics.AvgDiskWriteQueueLength,
		CurrentDiskQueueLength:  diskMetrics.CurrentDiskQueueLength,
		DiskBytesPersec:         diskMetrics.DiskBytesPersec,
		DiskReadBytesPersec:     diskMetrics.DiskReadBytesPersec,
		DiskReadsPersec:         diskMetrics.DiskReadsPersec,
		DiskTransfersPersec:     diskMetrics.DiskTransfersPersec,
		DiskWriteBytesPersec:    diskMetrics.DiskWriteBytesPersec,
		DiskWritesPersec:        diskMetrics.DiskWritesPersec,
		FreeMegabytes:           diskMetrics.FreeMegabytes,
		PercentDiskReadTime:     diskMetrics.PercentDiskReadTime,
		PercentDiskTime:         diskMetrics.PercentDiskTime,
		PercentDiskWriteTime:    diskMetrics.PercentDiskWriteTime,
		PercentFreeSpace:        diskMetrics.PercentFreeSpace,
		PercentIdleTime:         diskMetrics.PercentIdleTime,
		SplitIOPerSec:           diskMetrics.SplitIOPerSec,
	}
	return c.emitDiskMetrics(ctx, metrics, "logical", &dm)
}

func (c *Disk) emitPhysicalDiskMetrics(ctx context.Context, metrics *cgm.Metrics, diskMetrics *Win32_PerfFormattedData_PerfDisk_PhysicalDisk) error {
	c.logger.Debug().Str("disk", diskMetrics.Name).Msg("physical disk metrics")
	dm := genericDiskMetrics{
		Name:                    diskMetrics.Name,
		AvgDiskBytesPerRead:     diskMetrics.AvgDiskBytesPerRead,
		AvgDiskBytesPerTransfer: diskMetrics.AvgDiskBytesPerTransfer,
		AvgDiskBytesPerWrite:    diskMetrics.AvgDiskBytesPerWrite,
		AvgDiskQueueLength:      diskMetrics.AvgDiskQueueLength,
		AvgDiskReadQueueLength:  diskMetrics.AvgDiskReadQueueLength,
		AvgDisksecPerRead:       diskMetrics.AvgDisksecPerRead,
		AvgDisksecPerTransfer:   diskMetrics.AvgDisksecPerTransfer,
		AvgDisksecPerWrite:      diskMetrics.AvgDisksecPerWrite,
		AvgDiskWriteQueueLength: diskMetrics.AvgDiskWriteQueueLength,
		CurrentDiskQueueLength:  diskMetrics.CurrentDiskQueueLength,
		DiskBytesPersec:         diskMetrics.DiskBytesPersec,
		DiskReadBytesPersec:     diskMetrics.DiskReadBytesPersec,
		DiskReadsPersec:         diskMetrics.DiskReadsPersec,
		DiskTransfersPersec:     diskMetrics.DiskTransfersPersec,
		DiskWriteBytesPersec:    diskMetrics.DiskWriteBytesPersec,
		DiskWritesPersec:        diskMetrics.DiskWritesPersec,
		PercentDiskReadTime:     diskMetrics.PercentDiskReadTime,
		PercentDiskTime:         diskMetrics.PercentDiskTime,
		PercentDiskWriteTime:    diskMetrics.PercentDiskWriteTime,
		PercentIdleTime:         diskMetrics.PercentIdleTime,
		SplitIOPerSec:           diskMetrics.SplitIOPerSec,
	}
	return c.emitDiskMetrics(ctx, metrics, "physical", &dm)
}

func (c *Disk) emitDiskMetrics(ctx context.Context, metrics *cgm.Metrics, diskType string, diskMetrics *genericDiskMetrics) error {
	tagUnitsBytes := cgm.Tag{Category: "units", Value: "bytes"}
	tagUnitsMegabytes := cgm.Tag{Category: "units", Value: "megabytes"}
	tagUnitsOperations := cgm.Tag{Category: "units", Value: "operations"}
	tagUnitsPercent := cgm.Tag{Category: "units", Value: "percent"}

	metricTypeUint32 := "L"
	metricTypeUint64 := "I"

	// apply include/exclude to CLEAN item name
	diskName := c.cleanName(diskMetrics.Name)
	if c.exclude.MatchString(diskName) || !c.include.MatchString(diskName) {
		c.logger.Debug().Str("name", diskName).Msg("skipping, excluded")
		return nil
	}

	metricSuffix := ""
	if strings.Contains(diskMetrics.Name, totalName) {
		diskName = "all"
		metricSuffix = totalName
	}

	tagList := cgm.Tags{
		cgm.Tag{Category: "disk_type", Value: diskType},
		cgm.Tag{Category: "disk_name", Value: diskName},
	}

	var tagsBytes cgm.Tags
	tagsBytes = append(tagsBytes, tagList...)
	tagsBytes = append(tagsBytes, tagUnitsBytes)

	var tagsOperations cgm.Tags
	tagsOperations = append(tagsOperations, tagList...)
	tagsOperations = append(tagsOperations, tagUnitsOperations)

	var tagsPercent cgm.Tags
	tagsPercent = append(tagsPercent, tagList...)
	tagsPercent = append(tagsPercent, tagUnitsPercent)

	var tagsMegabytes cgm.Tags
	tagsMegabytes = append(tagsMegabytes, tagList...)
	tagsMegabytes = append(tagsMegabytes, tagUnitsMegabytes)

	if done(ctx) {
		return fmt.Errorf("context: %w", ctx.Err())
	}

	_ = c.addMetric(metrics, "", "AvgDiskBytesPerRead"+metricSuffix, metricTypeUint64, diskMetrics.AvgDiskBytesPerRead, tagsBytes)          // uint64
	_ = c.addMetric(metrics, "", "AvgDiskBytesPerTransfer"+metricSuffix, metricTypeUint64, diskMetrics.AvgDiskBytesPerTransfer, tagsBytes)  // uint64
	_ = c.addMetric(metrics, "", "AvgDiskBytesPerWrite"+metricSuffix, metricTypeUint64, diskMetrics.AvgDiskBytesPerWrite, tagsBytes)        // uint64
	_ = c.addMetric(metrics, "", "AvgDiskQueueLength"+metricSuffix, metricTypeUint64, diskMetrics.AvgDiskQueueLength, tagList)              // uint64
	_ = c.addMetric(metrics, "", "AvgDiskReadQueueLength"+metricSuffix, metricTypeUint64, diskMetrics.AvgDiskReadQueueLength, tagList)      // uint64
	_ = c.addMetric(metrics, "", "AvgDisksecPerRead"+metricSuffix, metricTypeUint32, diskMetrics.AvgDisksecPerRead, tagsOperations)         // uint32
	_ = c.addMetric(metrics, "", "AvgDisksecPerTransfer"+metricSuffix, metricTypeUint32, diskMetrics.AvgDisksecPerTransfer, tagsOperations) // uint32
	_ = c.addMetric(metrics, "", "AvgDisksecPerWrite"+metricSuffix, metricTypeUint32, diskMetrics.AvgDisksecPerWrite, tagsOperations)       // uint32
	_ = c.addMetric(metrics, "", "AvgDiskWriteQueueLength"+metricSuffix, metricTypeUint64, diskMetrics.AvgDiskWriteQueueLength, tagList)    // uint64
	_ = c.addMetric(metrics, "", "CurrentDiskQueueLength"+metricSuffix, metricTypeUint32, diskMetrics.CurrentDiskQueueLength, tagList)      // uint32
	_ = c.addMetric(metrics, "", "DiskBytesPersec"+metricSuffix, metricTypeUint64, diskMetrics.DiskBytesPersec, tagsBytes)                  // uint64
	_ = c.addMetric(metrics, "", "DiskReadBytesPersec"+metricSuffix, metricTypeUint64, diskMetrics.DiskReadBytesPersec, tagsBytes)          // uint64
	_ = c.addMetric(metrics, "", "DiskReadsPersec"+metricSuffix, metricTypeUint32, diskMetrics.DiskReadsPersec, tagList)                    // uint32
	_ = c.addMetric(metrics, "", "DiskTransfersPersec"+metricSuffix, metricTypeUint32, diskMetrics.DiskTransfersPersec, tagList)            // uint32
	_ = c.addMetric(metrics, "", "DiskWriteBytesPersec"+metricSuffix, metricTypeUint64, diskMetrics.DiskWriteBytesPersec, tagList)          // uint64
	_ = c.addMetric(metrics, "", "DiskWritesPersec"+metricSuffix, metricTypeUint64, diskMetrics.DiskWritesPersec, tagList)                  // uint64
	if diskType == "logical" {
		_ = c.addMetric(metrics, "", "FreeMegabytes"+metricSuffix, metricTypeUint32, diskMetrics.FreeMegabytes, tagsMegabytes) // uint32
	}
	_ = c.addMetric(metrics, "", "PercentDiskReadTime"+metricSuffix, metricTypeUint64, diskMetrics.PercentDiskReadTime, tagsPercent)   // uint64
	_ = c.addMetric(metrics, "", "PercentDiskTime"+metricSuffix, metricTypeUint64, diskMetrics.PercentDiskTime, tagsPercent)           // uint64
	_ = c.addMetric(metrics, "", "PercentDiskWriteTime"+metricSuffix, metricTypeUint64, diskMetrics.PercentDiskWriteTime, tagsPercent) // uint64
	if diskType == "logical" {
		_ = c.addMetric(metrics, "", "PercentFreeSpace"+metricSuffix, metricTypeUint32, diskMetrics.PercentFreeSpace, tagsPercent) // uint32
	}
	_ = c.addMetric(metrics, "", "PercentIdleTime"+metricSuffix, metricTypeUint64, diskMetrics.PercentIdleTime, tagsPercent) // uint64
	_ = c.addMetric(metrics, "", "SplitIOPerSec"+metricSuffix, metricTypeUint32, diskMetrics.SplitIOPerSec, tagsOperations)  // uint32

	return nil
}
