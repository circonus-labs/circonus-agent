// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build windows

package wmi

import (
	"fmt"
	"regexp"
	"strconv"
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

type genericDiskMetrics struct {
	Name string
	AvgDiskBytesPerRead     uint64
	AvgDiskBytesPerTransfer uint64
	AvgDiskBytesPerWrite    uint64
	AvgDiskQueueLength      uint64
	AvgDiskReadQueueLength  uint64
	AvgDisksecPerRead       uint32
	AvgDisksecPerTransfer   uint32
	AvgDisksecPerWrite      uint32
	AvgDiskWriteQueueLength uint64
	CurrentDiskQueueLength  uint32
	DiskBytesPersec         uint64
	DiskReadBytesPersec     uint64
	DiskReadsPersec         uint32
	DiskTransfersPersec     uint32
	DiskWriteBytesPersec    uint64
	DiskWritesPersec        uint64
	FreeMegabytes           uint32
	PercentDiskReadTime     uint64
	PercentDiskTime         uint64
	PercentDiskWriteTime    uint64
	PercentFreeSpace        uint32
	PercentIdleTime         uint64
	SplitIOPerSec           uint32
}

// Win32_PerfFormattedData_PerfDisk_LogicalDisk defines the metrics to collect
// https://technet.microsoft.com/en-ca/aa394261(v=vs.71)
type Win32_PerfFormattedData_PerfDisk_LogicalDisk struct {
	AvgDiskBytesPerRead     uint64
	AvgDiskBytesPerTransfer uint64
	AvgDiskBytesPerWrite    uint64
	AvgDiskQueueLength      uint64
	AvgDiskReadQueueLength  uint64
	AvgDisksecPerRead       uint32
	AvgDisksecPerTransfer   uint32
	AvgDisksecPerWrite      uint32
	AvgDiskWriteQueueLength uint64
	CurrentDiskQueueLength  uint32
	DiskBytesPersec         uint64
	DiskReadBytesPersec     uint64
	DiskReadsPersec         uint32
	DiskTransfersPersec     uint32
	DiskWriteBytesPersec    uint64
	DiskWritesPersec        uint64
	FreeMegabytes           uint32
	Name                    string
	PercentDiskReadTime     uint64
	PercentDiskTime         uint64
	PercentDiskWriteTime    uint64
	PercentFreeSpace        uint32
	PercentIdleTime         uint64
	SplitIOPerSec           uint32
}

// Win32_PerfFormattedData_PerfDisk_PhysicalDisk defines the metrics to collect
type Win32_PerfFormattedData_PerfDisk_PhysicalDisk struct {
	AvgDiskBytesPerRead     uint64
	AvgDiskBytesPerTransfer uint64
	AvgDiskBytesPerWrite    uint64
	AvgDiskQueueLength      uint64
	AvgDiskReadQueueLength  uint64
	AvgDisksecPerRead       uint32
	AvgDisksecPerTransfer   uint32
	AvgDisksecPerWrite      uint32
	AvgDiskWriteQueueLength uint64
	CurrentDiskQueueLength  uint32
	DiskBytesPersec         uint64
	DiskReadBytesPersec     uint64
	DiskReadsPersec         uint32
	DiskTransfersPersec     uint32
	DiskWriteBytesPersec    uint64
	DiskWritesPersec        uint64
	Name                    string
	PercentDiskReadTime     uint64
	PercentDiskTime         uint64
	PercentDiskWriteTime    uint64
	PercentIdleTime         uint64
	SplitIOPerSec           uint32
}

// Disk metrics from the Windows Management Interface (wmi)
type Disk struct {
	wmicommon
	logical  bool
	physical bool
	include  *regexp.Regexp
	exclude  *regexp.Regexp
}

// diskOptions defines what elements can be overridden in a config file
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

// NewDiskCollector creates new wmi collector
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
		return nil, errors.Wrapf(err, "%s config", c.pkgID)
	}

	c.logger.Debug().Interface("config", cfg).Msg("loaded config")

	if cfg.IncludeLogical != "" {
		logical, err := strconv.ParseBool(cfg.IncludeLogical)
		if err != nil {
			return nil, errors.Wrapf(err, "%s parsing disks", c.pkgID)
		}
		c.logical = logical
	}

	if cfg.IncludePhysical != "" {
		physical, err := strconv.ParseBool(cfg.IncludePhysical)
		if err != nil {
			return nil, errors.Wrapf(err, "%s parsing physical_disks", c.pkgID)
		}
		c.physical = physical
	}

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
		c.wmicommon.runTTL = dur
	}

	return &c, nil
}

// Collect metrics from the wmi resource
func (c *Disk) Collect() error {
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

	// tagUnitsBytes := cgm.Tag{Category: "units", Value: "bytes"}
	// // tagUnitsMegabytes := cgm.Tag{Category: "units", Value: "megabytes"}
	// tagUnitsOperations := cgm.Tag{Category: "units", Value: "operations"}
	// tagUnitsPercent := cgm.Tag{Category: "units", Value: "percent"}

	// metricTypeUint32 := "L"
	// metricTypeUint64 := "I"

	if c.logical {
		var dst []Win32_PerfFormattedData_PerfDisk_LogicalDisk
		qry := wmi.CreateQuery(dst, "")
		if err := wmi.Query(qry, &dst); err != nil {
			c.logger.Error().Err(err).Str("query", qry).Msg("wmi query error")
			c.setStatus(metrics, err)
			return errors.Wrap(err, c.pkgID)
		}

		for _, diskMetrics := range dst {
			_ = c.emitLogicalDiskMetrics(&metrics, &diskMetrics)
			// // apply include/exclude to CLEAN item name
			// if c.exclude.MatchString(itemName) || !c.include.MatchString(itemName) {
			// itemName := c.cleanName(item.Name)
			// 	continue
			// }

			// metricSuffix := ""
			// if strings.Contains(item.Name, totalName) {
			// 	itemName = "all"
			// 	metricSuffix = totalName
			// }
			// // // adjust prefix, add item name
			// // pfx := c.id + metricNameSeparator + "logical"
			// // if strings.Contains(item.Name, totalName) { // use the unclean name
			// // 	pfx += totalPrefix
			// // } else {
			// // 	pfx += metricNameSeparator + itemName
			// // }

			// tagList := cgm.Tags{
			// 	cgm.Tag{Category: "disk_type", Value: "logical"},
			// 	cgm.Tag{Category: "disk_name", Value: itemName},
			// }

			// var tagsBytes cgm.Tags
			// tagsBytes = append(tagsBytes, tagList...)
			// tagsBytes = append(tagsBytes, tagUnitsBytes)

			// var tagsOperations cgm.Tags
			// tagsOperations = append(tagsOperations, tagList...)
			// tagsOperations = append(tagsOperations, tagUnitsOperations)

			// var tagsPercent cgm.Tags
			// tagsPercent = append(tagsPercent, tagList...)
			// tagsPercent = append(tagsPercent, tagUnitsPercent)

			// var tagsMegabytes cgm.Tags
			// tagsMegabytes = append(tagsMegabytes, tagList...)
			// tagsMegabytes = append(tagsMegabytes, tagUnitsMegabytes)

			// _ = c.addMetric(&metrics, "", "AvgDiskBytesPerRead"+metricSuffic,"I",dst.AvgDiskBytesPerRead,tagsBytes)     // uint64
			// _ = c.addMetric(&metrics, "", "AvgDiskBytesPerTransfer"+metricSuffic,"I",dst.AvgDiskBytesPerTransfer,tagsBytes) // uint64
			// _ = c.addMetric(&metrics, "", "AvgDiskBytesPerWrite"+metricSuffic,"I",dst.AvgDiskBytesPerWrite,tagsBytes) // uint64
			// _ = c.addMetric(&metrics, "", "AvgDiskQueueLength"+metricSuffic,"I",dst.AvgDiskQueueLength,tagList)      // uint64
			// _ = c.addMetric(&metrics, "", "AvgDiskReadQueueLength"+metricSuffic,"I",dst.AvgDiskReadQueueLength,tagList)  // uint64
			// _ = c.addMetric(&metrics, "", "AvgDisksecPerRead"+metricSuffic,"L",dst.AvgDisksecPerRead,tagsOperations) // uint32
			// _ = c.addMetric(&metrics, "", "AvgDisksecPerTransfer"+metricSuffic,"L",dst.AvgDisksecPerTransfer,tagsOperations)   // uint32
			// _ = c.addMetric(&metrics, "", "AvgDisksecPerWrite"+metricSuffic,"L",dst.AvgDisksecPerWrite,tagsOperations)      // uint32
			// _ = c.addMetric(&metrics, "", "AvgDiskWriteQueueLength"+metricSuffic,"I",dst.AvgDiskWriteQueueLength,tagList) // uint64
			// _ = c.addMetric(&metrics, "", "CurrentDiskQueueLength"+metricSuffic,"L",dst.CurrentDiskQueueLength,tagList)  // uint32
			// _ = c.addMetric(&metrics, "", "DiskBytesPersec"+metricSuffic,"I",dst.DiskBytesPersec,tagsBytes)         // uint64
			// _ = c.addMetric(&metrics, "", "DiskReadBytesPersec"+metricSuffic,"I",dst.DiskReadBytesPersec,tagsBytes)     // uint64
			// _ = c.addMetric(&metrics, "", "DiskReadsPersec"+metricSuffic,"L",dst.DiskReadsPersec,tagList)         // uint32
			// _ = c.addMetric(&metrics, "", "DiskTransfersPersec"+metricSuffic,"L",dst.DiskTransfersPersec,tagList)    // uint32
			// _ = c.addMetric(&metrics, "", "DiskWriteBytesPersec"+metricSuffic,"I",dst.DiskWriteBytesPersec,tagList)    // uint64
			// _ = c.addMetric(&metrics, "", "DiskWritesPersec"+metricSuffic,"I",dst.DiskWritesPersec,tagList)        // uint64
			// _ = c.addMetric(&metrics, "", "FreeMegabytes"+metricSuffic,"L",dst.FreeMegabytes,tagsMegabytes)           // uint32
			// _ = c.addMetric(&metrics, "", "PercentDiskReadTime"+metricSuffic,"I",dst.PercentDiskReadTime,tagsPercent)     // uint64
			// _ = c.addMetric(&metrics, "", "PercentDiskTime"+metricSuffic,"I",dst.PercentDiskTime,tagsPercent)         // uint64
			// _ = c.addMetric(&metrics, "", "PercentDiskWriteTime"+metricSuffic,"I",dst.PercentDiskWriteTime, tagsPercent)    // uint64
			// _ = c.addMetric(&metrics, "", "PercentFreeSpace"+metricSuffic,"L",dst.PercentFreeSpace,tagsPercent)        // uint32
			// _ = c.addMetric(&metrics, "", "PercentIdleTime"+metricSuffic,"I",dst.PercentIdleTime,tagsPercent)         // uint64
			// _ = c.addMetric(&metrics, "", "SplitIOPerSec"+metricSuffic,"L",dst.SplitIOPerSec,tagsOperations)           // uint32

			// // d := structs.Map(item)
			// // for name, val := range d {
			// // 	if name == nameFieldName {
			// // 		continue
			// // 	}
			// // 	_ = c.addMetric(&metrics, "", name+metricSuffix, "L", val, tagList)
			// // }
		}
	}

	if c.physical {
		var dst []Win32_PerfFormattedData_PerfDisk_PhysicalDisk
		qry := wmi.CreateQuery(dst, "")
		if err := wmi.Query(qry, &dst); err != nil {
			c.logger.Error().Err(err).Str("query", qry).Msg("wmi query error")
			c.setStatus(metrics, err)
			return errors.Wrap(err, c.pkgID)
		}

		for _, diskMetrics := range dst {
			_ = c.emitPhysicalDiskMetrics(&metrics, &diskMetrics)
			// // apply include/exclude to CLEAN item name
			// itemName := c.cleanName(item.Name)
			// if c.exclude.MatchString(itemName) || !c.include.MatchString(itemName) {
			// 	continue
			// }

			// metricSuffix := ""
			// if strings.Contains(item.Name, totalName) {
			// 	itemName = "all"
			// 	metricSuffix = totalName
			// }

			// // // adjust prefix, add item name
			// // pfx := c.id + metricNameSeparator + "physical"
			// // if strings.Contains(item.Name, totalName) { // use the unclean name
			// // 	pfx += totalPrefix
			// // } else {
			// // 	pfx += metricNameSeparator + itemName
			// // }

			// tagList := cgm.Tags{
			// 	cgm.Tag{Category: "disk_type", Value: "physical"},
			// 	cgm.Tag{Category: "disk_name", Value: itemName},
			// }

			// var tagsBytes cgm.Tags
			// tagsBytes = append(tagsBytes, tagList...)
			// tagsBytes = append(tagsBytes, tagUnitsBytes)

			// var tagsOperations cgm.Tags
			// tagsOperations = append(tagsOperations, tagList...)
			// tagsOperations = append(tagsOperations, tagUnitsOperations)

			// var tagsPercent cgm.Tags
			// tagsPercent = append(tagsPercent, tagList...)
			// tagsPercent = append(tagsPercent, tagUnitsPercent)

			// // var tagsMegabytes cgm.Tags
			// // tagsMegabytes = append(tagsMegabytes, tagList...)
			// // tagsMegabytes = append(tagsMegabytes, tagUnitsMegabytes)

			// _ = c.addMetric(&metrics, "", "AvgDiskBytesPerRead"+metricSuffix, metricTypeUint64, item.AvgDiskBytesPerRead, tagsBytes)  // uint64
			// _ = c.addMetric(&metrics, "", "AvgDiskBytesPerTransfer"+metricSuffix, "I", item.AvgDiskBytesPerTransfer, tagsBytes)       // uint64
			// _ = c.addMetric(&metrics, "", "AvgDiskBytesPerWrite"+metricSuffix, "I", item.AvgDiskBytesPerWrite, tagsBytes)             // uint64
			// _ = c.addMetric(&metrics, "", "AvgDiskQueueLength"+metricSuffix, "I", item.AvgDiskQueueLength, tagList)                   // uint64
			// _ = c.addMetric(&metrics, "", "AvgDiskReadQueueLength"+metricSuffix, "I", item.AvgDiskReadQueueLength, tagList)           // uint64
			// _ = c.addMetric(&metrics, "", "AvgDisksecPerRead"+metricSuffix, metricTypeUint32, item.AvgDisksecPerRead, tagsOperations) // uint32
			// _ = c.addMetric(&metrics, "", "AvgDisksecPerTransfer"+metricSuffix, "L", item.AvgDisksecPerTransfer, tagsOperations)      // uint32
			// _ = c.addMetric(&metrics, "", "AvgDisksecPerWrite"+metricSuffix, "L", item.AvgDisksecPerWrite, tagsOperations)            // uint32
			// _ = c.addMetric(&metrics, "", "AvgDiskWriteQueueLength"+metricSuffix, "I", item.AvgDiskWriteQueueLength, tagList)         // uint64
			// _ = c.addMetric(&metrics, "", "CurrentDiskQueueLength"+metricSuffix, "L", item.CurrentDiskQueueLength, tagList)           // uint32
			// _ = c.addMetric(&metrics, "", "DiskBytesPersec"+metricSuffix, "I", item.DiskBytesPersec, tagsBytes)                       // uint64
			// _ = c.addMetric(&metrics, "", "DiskReadBytesPersec"+metricSuffix, "I", item.DiskReadBytesPersec, tagsBytes)               // uint64
			// _ = c.addMetric(&metrics, "", "DiskReadsPersec"+metricSuffix, "L", item.DiskReadsPersec, tagList)                         // uint32
			// _ = c.addMetric(&metrics, "", "DiskTransfersPersec"+metricSuffix, "L", item.DiskTransfersPersec, tagList)                 // uint32
			// _ = c.addMetric(&metrics, "", "DiskWriteBytesPersec"+metricSuffix, "I", item.DiskWriteBytesPersec, tagList)               // uint64
			// _ = c.addMetric(&metrics, "", "DiskWritesPersec"+metricSuffix, "I", item.DiskWritesPersec, tagList)                       // uint64
			// // _ = c.addMetric(&metrics, "", "FreeMegabytes"+metricSuffix, "L", item.FreeMegabytes, tagsMegabytes)                  // uint32
			// _ = c.addMetric(&metrics, "", "PercentDiskReadTime"+metricSuffix, "I", item.PercentDiskReadTime, tagsPercent)   // uint64
			// _ = c.addMetric(&metrics, "", "PercentDiskTime"+metricSuffix, "I", item.PercentDiskTime, tagsPercent)           // uint64
			// _ = c.addMetric(&metrics, "", "PercentDiskWriteTime"+metricSuffix, "I", item.PercentDiskWriteTime, tagsPercent) // uint64
			// // _ = c.addMetric(&metrics, "", "PercentFreeSpace"+metricSuffix, "L", item.PercentFreeSpace, tagsPercent)              // uint32
			// _ = c.addMetric(&metrics, "", "PercentIdleTime"+metricSuffix, "I", item.PercentIdleTime, tagsPercent) // uint64
			// _ = c.addMetric(&metrics, "", "SplitIOPerSec"+metricSuffix, "L", item.SplitIOPerSec, tagsOperations)  // uint32

			// // d := structs.Map(item)
			// // for name, val := range d {
			// // 	if name == nameFieldName {
			// // 		continue
			// // 	}
			// // 	_ = c.addMetric(&metrics, pfx, name, "L", val, cgm.Tags{})
			// // }
		}
	}

	c.setStatus(metrics, nil)
	return nil
}

func (c *Disk) emitLogicalDiskMetrics(metrics *cgm.Metrics, diskMetrics *Win32_PerfFormattedData_PerfDisk_LogicalDisk) error {
	dm := genericDiskMetrics{
		Name: diskMetrics.Name,
		AvgDiskBytesPerRead: diskMetrics.AvgDiskBytesPerRead,
		AvgDiskBytesPerTransfer: diskMetrics.AvgDiskBytesPerTransfer,
		AvgDiskBytesPerWrite: diskMetrics.AvgDiskBytesPerWrite,
		AvgDiskQueueLength: diskMetrics.AvgDiskQueueLength,
		AvgDiskReadQueueLength: diskMetrics.AvgDiskReadQueueLength,
		AvgDisksecPerRead: diskMetrics.AvgDisksecPerRead,
		AvgDisksecPerTransfer: diskMetrics.AvgDisksecPerTransfer,
		AvgDisksecPerWrite: diskMetrics.AvgDisksecPerWrite,
		AvgDiskWriteQueueLength: diskMetrics.AvgDiskWriteQueueLength,
		CurrentDiskQueueLength: diskMetrics.CurrentDiskQueueLength,
		DiskBytesPersec: diskMetrics.DiskBytesPersec,
		DiskReadBytesPersec: diskMetrics.DiskReadBytesPersec,
		DiskReadsPersec: diskMetrics.DiskReadsPersec,
		DiskTransfersPersec: diskMetrics.DiskTransfersPersec,
		DiskWriteBytesPersec: diskMetrics.DiskWriteBytesPersec,
		DiskWritesPersec: diskMetrics.DiskWritesPersec,
		FreeMegabytes: diskMetrics.FreeMegabytes,
		PercentDiskReadTime: diskMetrics.PercentDiskReadTime,
		PercentDiskTime: diskMetrics.PercentDiskTime,
		PercentDiskWriteTime: diskMetrics.PercentDiskWriteTime,
		PercentFreeSpace: diskMetrics.PercentFreeSpace,
		PercentIdleTime: diskMetrics.PercentIdleTime,
		SplitIOPerSec: diskMetrics.SplitIOPerSec,
	}
	return c.emitDiskMetrics(&metrics, "logical", &dm)
}

func (c *Disk) emitPhysicalDiskMetrics(metrics *cgm.Metrics, diskMetrics *Win32_PerfFormattedData_PerfDisk_LogicalDisk) error {
	dm := genericDiskMetrics{
		AvgDiskBytesPerRead: diskMetrics.AvgDiskBytesPerRead,
		AvgDiskBytesPerTransfer: diskMetrics.AvgDiskBytesPerTransfer,
		AvgDiskBytesPerWrite: diskMetrics.AvgDiskBytesPerWrite,
		AvgDiskQueueLength: diskMetrics.AvgDiskQueueLength,
		AvgDiskReadQueueLength: diskMetrics.AvgDiskReadQueueLength,
		AvgDisksecPerRead: diskMetrics.AvgDisksecPerRead,
		AvgDisksecPerTransfer: diskMetrics.AvgDisksecPerTransfer,
		AvgDisksecPerWrite: diskMetrics.AvgDisksecPerWrite,
		AvgDiskWriteQueueLength: diskMetrics.AvgDiskWriteQueueLength,
		CurrentDiskQueueLength: diskMetrics.CurrentDiskQueueLength,
		DiskBytesPersec: diskMetrics.DiskBytesPersec,
		DiskReadBytesPersec: diskMetrics.DiskReadBytesPersec,
		DiskReadsPersec: diskMetrics.DiskReadsPersec,
		DiskTransfersPersec: diskMetrics.DiskTransfersPersec,
		DiskWriteBytesPersec: diskMetrics.DiskWriteBytesPersec,
		DiskWritesPersec: diskMetrics.DiskWritesPersec,
		PercentDiskReadTime: diskMetrics.PercentDiskReadTime,
		PercentDiskTime: diskMetrics.PercentDiskTime,
		PercentDiskWriteTime: diskMetrics.PercentDiskWriteTime,
		PercentIdleTime: diskMetrics.PercentIdleTime,
		SplitIOPerSec: diskMetrics.SplitIOPerSec,
	}
	return c.emitDiskMetrics(&metrics, "physical", &dm)
}

func (c *Disk) emitDiskMetrics(metrics *cgm.Metrics, diskType string, diskMetrics *genericDiskMetrics) error {
	tagUnitsBytes := cgm.Tag{Category: "units", Value: "bytes"}
	tagUnitsMegabytes := cgm.Tag{Category: "units", Value: "megabytes"}
	tagUnitsOperations := cgm.Tag{Category: "units", Value: "operations"}
	tagUnitsPercent := cgm.Tag{Category: "units", Value: "percent"}

	metricTypeUint32 := "L"
	metricTypeUint64 := "I"

	// apply include/exclude to CLEAN item name
	diskName := c.cleanName(diskMetrics.Name)
	if c.exclude.MatchString(diskName) || !c.include.MatchString(diskName) {
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
		_ = c.addMetric(metrics, "", "FreeMegabytes"+metricSuffix, metricTypeUint32, diskMetrics.FreeMegabytes, tagsMegabytes)                  // uint32
	}
	_ = c.addMetric(metrics, "", "PercentDiskReadTime"+metricSuffix, metricTypeUint64, diskMetrics.PercentDiskReadTime, tagsPercent)        // uint64
	_ = c.addMetric(metrics, "", "PercentDiskTime"+metricSuffix, metricTypeUint64, diskMetrics.PercentDiskTime, tagsPercent)                // uint64
	_ = c.addMetric(metrics, "", "PercentDiskWriteTime"+metricSuffix, metricTypeUint64, diskMetrics.PercentDiskWriteTime, tagsPercent)      // uint64
	if diskType == "logical" {
		_ = c.addMetric(metrics, "", "PercentFreeSpace"+metricSuffix, metricTypeUint32, diskMetrics.PercentFreeSpace, tagsPercent)              // uint32
	}
	_ = c.addMetric(metrics, "", "PercentIdleTime"+metricSuffix, metricTypeUint64, diskMetrics.PercentIdleTime, tagsPercent)                // uint64
	_ = c.addMetric(metrics, "", "SplitIOPerSec"+metricSuffix, metricTypeUint32, diskMetrics.SplitIOPerSec, tagsOperations)                 // uint32

	return nil
}
