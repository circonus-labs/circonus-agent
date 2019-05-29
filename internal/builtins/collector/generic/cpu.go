// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/cpu"
)

// CPU metrics from psutils
type CPU struct {
	gencommon
	reportAllCPUs bool // OPT report all cpus (vs just total) may be overridden in config file
}

// cpuOptions defines what elements can be overridden in a config file
type cpuOptions struct {
	// common
	ID     string `json:"id" toml:"id" yaml:"id"`
	RunTTL string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`

	// collector specific
	AllCPU string `json:"report_all_cpus" toml:"report_all_cpus" yaml:"report_all_cpus"`
}

// NewCPUCollector creates new psutils cpu collector
func NewCPUCollector(cfgBaseName string, parentLogger zerolog.Logger) (collector.Collector, error) {
	c := CPU{}
	c.id = NameCPU
	c.pkgID = PackageName + "." + c.id
	c.logger = parentLogger.With().Str("id", c.id).Logger()
	c.reportAllCPUs = false
	c.baseTags = tags.FromList(tags.GetBaseTags())

	var opts cpuOptions
	err := config.LoadConfigFile(cfgBaseName, &opts)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, errors.Wrapf(err, "%s config", c.pkgID)
	}

	c.logger.Debug().Interface("config", opts).Msg("loaded config")

	if opts.AllCPU != "" {
		rpt, err := strconv.ParseBool(opts.AllCPU)
		if err != nil {
			return nil, errors.Wrapf(err, "%s parsing report_all_cpus", c.pkgID)
		}
		c.reportAllCPUs = rpt
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

// Collect cpu metrics
func (c *CPU) Collect() error {
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
	pcts, err := cpu.Percent(time.Duration(0), c.reportAllCPUs)
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting metrics, cpu%")
	} else {
		if !c.reportAllCPUs && len(pcts) == 1 {
			var tagList tags.Tags
			tagList = append(tagList, moduleTags...)
			tagList = append(tagList, tags.Tag{Category: "units", Value: "percent"})
			_ = c.addMetric(&metrics, "used", "n", pcts[0], tagList)
		} else {
			for idx, pct := range pcts {
				var tagList tags.Tags
				tagList = append(tagList, moduleTags...)
				tagList = append(tagList, tags.Tags{
					tags.Tag{Category: "cpu", Value: fmt.Sprintf("%d", idx)},
					tags.Tag{Category: "units", Value: "percent"},
				}...)
				_ = c.addMetric(&metrics, "used", "n", pct, tagList)
			}
		}
	}

	var cpuTags tags.Tags
	cpuTags = append(cpuTags, moduleTags...)
	cpuTags = append(cpuTags, tags.Tags{
		tags.Tag{Category: "units", Value: "jiffies"},
	}...)

	ts, err := cpu.Times(c.reportAllCPUs)
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting metrics, cpu times")
	} else {
		if !c.reportAllCPUs && len(ts) == 1 {
			_ = c.addMetric(&metrics, "user", "n", ts[0].User, cpuTags)
			_ = c.addMetric(&metrics, "system", "n", ts[0].System, cpuTags)
			_ = c.addMetric(&metrics, "idle", "n", ts[0].Idle, cpuTags)
			_ = c.addMetric(&metrics, "nice", "n", ts[0].Nice, cpuTags)
			_ = c.addMetric(&metrics, "iowait", "n", ts[0].Iowait, cpuTags)
			_ = c.addMetric(&metrics, "irq", "n", ts[0].Irq, cpuTags)
			_ = c.addMetric(&metrics, "soft_irq", "n", ts[0].Softirq, cpuTags)
			_ = c.addMetric(&metrics, "steal", "n", ts[0].Steal, cpuTags)
			_ = c.addMetric(&metrics, "guest", "n", ts[0].Guest, cpuTags)
			_ = c.addMetric(&metrics, "guest_nice", "n", ts[0].GuestNice, cpuTags)
			_ = c.addMetric(&metrics, "stolen", "n", ts[0].Stolen, cpuTags)
		} else {
			for idx, v := range ts {
				var tagList tags.Tags
				tagList = append(tagList, cpuTags...)
				tagList = append(tagList, tags.Tag{Category: "cpu", Value: fmt.Sprintf("%d", idx)})
				_ = c.addMetric(&metrics, "user", "n", v.User, tagList)
				_ = c.addMetric(&metrics, "system", "n", v.System, tagList)
				_ = c.addMetric(&metrics, "idle", "n", v.Idle, tagList)
				_ = c.addMetric(&metrics, "nice", "n", v.Nice, tagList)
				_ = c.addMetric(&metrics, "iowait", "n", v.Iowait, tagList)
				_ = c.addMetric(&metrics, "irq", "n", v.Irq, tagList)
				_ = c.addMetric(&metrics, "soft_irq", "n", v.Softirq, tagList)
				_ = c.addMetric(&metrics, "steal", "n", v.Steal, tagList)
				_ = c.addMetric(&metrics, "guest", "n", v.Guest, tagList)
				_ = c.addMetric(&metrics, "guest_nice", "n", v.GuestNice, tagList)
				_ = c.addMetric(&metrics, "stolen", "n", v.Stolen, tagList)
			}
		}
	}

	c.setStatus(metrics, nil)
	return nil
}
