// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/disk"
)

// Disk metrics from the Linux ProcFS
type Disk struct {
	gencommon
	ioDevices []string
}

// DiskOptions defines what elements can be overridden in a config file
type DiskOptions struct {
	// common
	ID     string `json:"id" toml:"id" yaml:"id"`
	RunTTL string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`

	// collector specific
	IODevices []string `json:"io_devices" toml:"io_devices" yaml:"io_devices"`
}

// NewDiskCollector creates new psutils disk collector
func NewDiskCollector(cfgBaseName string, parentLogger zerolog.Logger) (collector.Collector, error) {
	c := Disk{}
	c.id = NameDisk
	c.pkgID = PackageName + "." + c.id
	c.logger = parentLogger.With().Str("id", c.id).Logger()
	c.ioDevices = []string{}
	c.baseTags = tags.FromList(tags.GetBaseTags())

	var opts DiskOptions
	err := config.LoadConfigFile(cfgBaseName, &opts)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, errors.Wrapf(err, "%s config", c.pkgID)
	}

	c.logger.Debug().Str("base", cfgBaseName).Interface("config", opts).Msg("loaded config")

	if len(opts.IODevices) > 0 {
		c.ioDevices = opts.IODevices
	}

	if opts.ID != "" {
		c.id = opts.ID
	}

	if opts.RunTTL != "" {
		dur, err := time.ParseDuration(opts.RunTTL)
		if err != nil {
			return nil, errors.Wrapf(err, "%s parsing run_ttl", c.pkgID)
		}
		c.runTTL = dur
	}

	return &c, nil
}

// Collect disk device metrics
func (c *Disk) Collect() error {
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

	moduleTags := tags.Tags{
		tags.Tag{Category: release.NAME + "-module", Value: c.id},
	}

	metrics := cgm.Metrics{}
	ios, err := disk.IOCounters(c.ioDevices...)
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting disk io counter metrics")
	} else {
		for device, counters := range ios {
			var diskTags tags.Tags
			diskTags = append(diskTags, moduleTags...)
			diskTags = append(diskTags, tags.Tag{Category: "device", Value: device})

			{
				// units:reads
				var tagList tags.Tags
				tagList = append(tagList, diskTags...)
				tagList = append(tagList, tags.Tag{Category: "units", Value: "reads"})
				_ = c.addMetric(&metrics, "reads", "L", counters.ReadCount, tagList)
			}
			{
				// units:writes
				var tagList tags.Tags
				tagList = append(tagList, diskTags...)
				tagList = append(tagList, tags.Tag{Category: "units", Value: "writes"})
				_ = c.addMetric(&metrics, "writes", "L", counters.WriteCount, tagList)
			}
			{
				// units:merged_reads
				var tagList tags.Tags
				tagList = append(tagList, diskTags...)
				tagList = append(tagList, tags.Tag{Category: "units", Value: "merged_reads"})
				_ = c.addMetric(&metrics, "merged_reads", "L", counters.MergedReadCount, tagList)
			}
			{
				// units:merged_writes
				var tagList tags.Tags
				tagList = append(tagList, diskTags...)
				tagList = append(tagList, tags.Tag{Category: "units", Value: "merged_writes"})
				_ = c.addMetric(&metrics, "merged_writes", "L", counters.MergedWriteCount, tagList)
			}
			{
				// units:bytes
				var tagList tags.Tags
				tagList = append(tagList, diskTags...)
				tagList = append(tagList, tags.Tag{Category: "units", Value: "bytes"})
				_ = c.addMetric(&metrics, "reads", "L", counters.ReadBytes, tagList)
				_ = c.addMetric(&metrics, "writes", "L", counters.WriteBytes, tagList)
			}
			{
				// units:milliseconds
				var tagList tags.Tags
				tagList = append(tagList, diskTags...)
				tagList = append(tagList, tags.Tag{Category: "units", Value: "milliseconds"})
				_ = c.addMetric(&metrics, "read_time", "L", counters.ReadTime, tagList)
				_ = c.addMetric(&metrics, "write_time", "L", counters.WriteTime, tagList)
				_ = c.addMetric(&metrics, "io_time", "L", counters.IoTime, tagList)
				_ = c.addMetric(&metrics, "weighted_io_time", "L", counters.WeightedIO, tagList)
			}
			{
				// units:iops_in_progress
				var tagList tags.Tags
				tagList = append(tagList, diskTags...)
				tagList = append(tagList, tags.Tag{Category: "units", Value: "iops_in_progress"})
				_ = c.addMetric(&metrics, "iops_in_progress", "L", counters.IopsInProgress, tagList)
			}

		}
	}

	c.setStatus(metrics, nil)
	return nil
}
