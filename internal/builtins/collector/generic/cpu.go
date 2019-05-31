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

	metrics := cgm.Metrics{}
	pcts, err := cpu.Percent(time.Duration(0), c.reportAllCPUs)
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting metrics, cpu%")
	} else {
		metricName := "cpu_used"
		metricType := "n"
		tagUnitsPercent := tags.Tag{Category: "units", Value: "percent"}
		if !c.reportAllCPUs && len(pcts) == 1 {
			tagList := tags.Tags{tagUnitsPercent}
			_ = c.addMetric(&metrics, metricName, metricType, pcts[0], tagList)
		} else {
			for idx, pct := range pcts {
				tagList := tags.Tags{
					tags.Tag{Category: "cpu", Value: fmt.Sprintf("%d", idx)},
				}
				tagList = append(tagList, tagUnitsPercent)
				_ = c.addMetric(&metrics, metricName, metricType, pct, tagList)
			}
		}
	}

	ts, err := cpu.Times(c.reportAllCPUs)
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting metrics, cpu times")
		c.setStatus(metrics, nil)
		return nil
	}

	tagUnitsCentiseconds := tags.Tag{Category: "units", Value: "centiseconds"} // aka jiffies
	if !c.reportAllCPUs && len(ts) == 1 {
		tagList := tags.Tags{tagUnitsCentiseconds}
		_ = c.addMetric(&metrics, "cpu_user", "n", ts[0].User, tagList)
		_ = c.addMetric(&metrics, "cpu_system", "n", ts[0].System, tagList)
		_ = c.addMetric(&metrics, "cpu_idle", "n", ts[0].Idle, tagList)
		_ = c.addMetric(&metrics, "cpu_nice", "n", ts[0].Nice, tagList)
		_ = c.addMetric(&metrics, "cpu_iowait", "n", ts[0].Iowait, tagList)
		_ = c.addMetric(&metrics, "cpu_irq", "n", ts[0].Irq, tagList)
		_ = c.addMetric(&metrics, "cpu_soft_irq", "n", ts[0].Softirq, tagList)
		_ = c.addMetric(&metrics, "cpu_steal", "n", ts[0].Steal, tagList)
		_ = c.addMetric(&metrics, "cpu_guest", "n", ts[0].Guest, tagList)
		_ = c.addMetric(&metrics, "cpu_guest_nice", "n", ts[0].GuestNice, tagList)
		_ = c.addMetric(&metrics, "cpu_stolen", "n", ts[0].Stolen, tagList)
	} else {
		for idx, v := range ts {
			tagList := tags.Tags{
				tags.Tag{Category: "cpu", Value: fmt.Sprintf("%d", idx)},
			}
			tagList = append(tagList, tagUnitsCentiseconds)
			_ = c.addMetric(&metrics, "cpu_user", "n", v.User, tagList)
			_ = c.addMetric(&metrics, "cpu_system", "n", v.System, tagList)
			_ = c.addMetric(&metrics, "cpu_idle", "n", v.Idle, tagList)
			_ = c.addMetric(&metrics, "cpu_nice", "n", v.Nice, tagList)
			_ = c.addMetric(&metrics, "cpu_iowait", "n", v.Iowait, tagList)
			_ = c.addMetric(&metrics, "cpu_irq", "n", v.Irq, tagList)
			_ = c.addMetric(&metrics, "cpu_soft_irq", "n", v.Softirq, tagList)
			_ = c.addMetric(&metrics, "cpu_steal", "n", v.Steal, tagList)
			_ = c.addMetric(&metrics, "cpu_guest", "n", v.Guest, tagList)
			_ = c.addMetric(&metrics, "cpu_guest_nice", "n", v.GuestNice, tagList)
			_ = c.addMetric(&metrics, "cpu_stolen", "n", v.Stolen, tagList)
		}
	}

	c.setStatus(metrics, nil)
	return nil
}
