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
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/disk"
)

// FS metrics from the Linux ProcFS
type FS struct {
	common
	includeFS *regexp.Regexp
	excludeFS *regexp.Regexp
}

// fsOptions defines what elements can be overridden in a config file
type fsOptions struct {
	// common
	ID                   string   `json:"id" toml:"id" yaml:"id"`
	MetricsEnabled       []string `json:"metrics_enabled" toml:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsDisabled      []string `json:"metrics_disabled" toml:"metrics_disabled" yaml:"metrics_disabled"`
	MetricsDefaultStatus string   `json:"metrics_default_status" toml:"metrics_default_status" toml:"metrics_default_status"`
	RunTTL               string   `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`

	// collector specific
	IncludeRegexFS string `json:"include_fs_regex" toml:"include_fs_regex" yaml:"include_fs_regex"`
	ExcludeRegexFS string `json:"exclude_fs_regex" toml:"exclude_fs_regex" yaml:"exclude_fs_regex"`
}

// NewDiskFSCollector creates new psutils disk collector
func NewDiskFSCollector(cfgBaseName string) (collector.Collector, error) {
	c := FS{}
	c.id = FS_NAME
	c.pkgID = LOG_PREFIX + c.id
	c.logger = log.With().Str("pkg", c.pkgID).Logger()
	c.metricStatus = map[string]bool{}
	c.metricDefaultActive = true

	c.includeFS = defaultIncludeRegex
	c.excludeFS = defaultExcludeRegex

	var opts fsOptions
	err := config.LoadConfigFile(cfgBaseName, &opts)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, errors.Wrapf(err, "%s config", c.pkgID)
	}

	c.logger.Debug().Str("base", cfgBaseName).Interface("config", opts).Msg("loaded config")

	if opts.IncludeRegexFS != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, opts.IncludeRegexFS))
		if err != nil {
			return nil, errors.Wrapf(err, "%s compiling include FS regex", c.pkgID)
		}
		c.includeFS = rx
	}

	if opts.ExcludeRegexFS != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, opts.ExcludeRegexFS))
		if err != nil {
			return nil, errors.Wrapf(err, "%s compiling exclude FS regex", c.pkgID)
		}
		c.excludeFS = rx
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

// Collect disk fs metrics
func (c *FS) Collect() error {
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

	partitions, err := disk.Partitions(false)
	if err != nil {
		c.logger.Warn().Err(err).Str("id", c.id).Msg("collecting disk filesystem/partition metrics")
	} else {
		for _, partition := range partitions {
			if c.excludeFS.MatchString(partition.Mountpoint) || !c.includeFS.MatchString(partition.Mountpoint) {
				c.logger.Debug().Str("mount_point", partition.Mountpoint).Msg("excluded FS, ignoring")
				continue
			}

			usage, err := disk.Usage(partition.Mountpoint)
			if err != nil {
				c.logger.Warn().Err(err).Str("mount_point", partition.Mountpoint).Msg("collecting disk usage")
				continue
			}

			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", partition.Mountpoint, metricNameSeparator, "total"), "L", usage.Total)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", partition.Mountpoint, metricNameSeparator, "free"), "L", usage.Free)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", partition.Mountpoint, metricNameSeparator, "used"), "L", usage.Used)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", partition.Mountpoint, metricNameSeparator, "used_pct"), "n", usage.UsedPercent)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", partition.Mountpoint, metricNameSeparator, "inodes_total"), "L", usage.InodesTotal)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", partition.Mountpoint, metricNameSeparator, "inodes_used"), "L", usage.InodesUsed)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", partition.Mountpoint, metricNameSeparator, "inodes_free"), "L", usage.InodesFree)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", partition.Mountpoint, metricNameSeparator, "inodes_used_pct"), "n", usage.InodesUsedPercent)
		}
	}

	c.setStatus(metrics, nil)
	return nil
}
