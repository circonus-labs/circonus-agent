// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
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
	ID                   string   `json:"id" toml:"id" yaml:"id"`
	MetricsEnabled       []string `json:"metrics_enabled" toml:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsDisabled      []string `json:"metrics_disabled" toml:"metrics_disabled" yaml:"metrics_disabled"`
	MetricsDefaultStatus string   `json:"metrics_default_status" toml:"metrics_default_status" toml:"metrics_default_status"`
	RunTTL               string   `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`

	// collector specific
	IODevices []string `json:"io_devices" toml:"io_devices" yaml:"io_devices"`
}

// NewDiskCollector creates new psutils disk collector
func NewDiskCollector(cfgBaseName string) (collector.Collector, error) {
	c := Disk{}
	c.id = DISK_NAME
	c.pkgID = PKG_NAME + "." + c.id
	c.logger = log.With().Str("pkg", PKG_NAME).Str("id", c.id).Logger()
	c.metricStatus = map[string]bool{}
	c.metricDefaultActive = true
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

	if len(opts.MetricsEnabled) > 0 {
		for _, name := range opts.MetricsEnabled {
			c.metricStatus[name] = true
		}
	}
	if len(opts.MetricsDisabled) > 0 {
		for _, name := range opts.MetricsDisabled {
			c.metricStatus[name] = false
		}
	}

	if opts.MetricsDefaultStatus != "" {
		if ok, _ := regexp.MatchString(`^(enabled|disabled)$`, strings.ToLower(opts.MetricsDefaultStatus)); ok {
			c.metricDefaultActive = strings.ToLower(opts.MetricsDefaultStatus) == metricStatusEnabled
		} else {
			return nil, errors.Errorf("%s invalid metric default status (%s)", c.pkgID, opts.MetricsDefaultStatus)
		}
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

	metrics := cgm.Metrics{}
	ios, err := disk.IOCounters(c.ioDevices...)
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting disk io counter metrics")
	} else {
		for device, counters := range ios {
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", device, metricNameSeparator, "read_count"), "L", counters.ReadCount)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", device, metricNameSeparator, "merged_read_count"), "L", counters.MergedReadCount)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", device, metricNameSeparator, "write_count"), "L", counters.WriteCount)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", device, metricNameSeparator, "merged_write_count"), "L", counters.MergedWriteCount)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", device, metricNameSeparator, "read_bytes"), "L", counters.ReadBytes)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", device, metricNameSeparator, "write_bytes"), "L", counters.WriteBytes)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", device, metricNameSeparator, "read_time"), "L", counters.ReadTime)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", device, metricNameSeparator, "write_time"), "L", counters.WriteTime)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", device, metricNameSeparator, "iops_in_progress"), "L", counters.IopsInProgress)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", device, metricNameSeparator, "io_time"), "L", counters.IoTime)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", device, metricNameSeparator, "weighted_io"), "L", counters.WeightedIO)
		}
	}

	c.setStatus(metrics, nil)
	return nil
}
