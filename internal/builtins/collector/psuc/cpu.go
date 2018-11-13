// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package psuc

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/cpu"
)

// CPU metrics from psutils
type CPU struct {
	common
	reportAllCPUs bool // OPT report all cpus (vs just total) may be overridden in config file
}

// cpuOptions defines what elements can be overridden in a config file
type cpuOptions struct {
	// common
	ID                   string   `json:"id" toml:"id" yaml:"id"`
	MetricsEnabled       []string `json:"metrics_enabled" toml:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsDisabled      []string `json:"metrics_disabled" toml:"metrics_disabled" yaml:"metrics_disabled"`
	MetricsDefaultStatus string   `json:"metrics_default_status" toml:"metrics_default_status" toml:"metrics_default_status"`
	RunTTL               string   `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`

	// collector specific
	AllCPU string `json:"report_all_cpus" toml:"report_all_cpus" yaml:"report_all_cpus"`
}

// NewCPUCollector creates new psutils cpu collector
func NewCPUCollector(cfgBaseName string) (collector.Collector, error) {
	c := CPU{}
	c.id = CPU_NAME
	c.pkgID = LOG_PREFIX + c.id
	c.logger = log.With().Str("pkg", c.pkgID).Logger()
	c.metricStatus = map[string]bool{}
	c.metricDefaultActive = true
	c.reportAllCPUs = false

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

	pcts, err := cpu.Percent(time.Duration(0), true)
	if err != nil {
		c.logger.Warn().Err(err).Str("id", c.id).Msg("collecting metrics, cpu%")
	} else {
		for idx, v := range pcts {
			c.addMetric(&metrics, c.id, fmt.Sprintf("%d", idx), "n", v)
		}
	}

	ts, err := cpu.Times(c.reportAllCPUs)
	if err != nil {
		c.logger.Warn().Err(err).Str("id", c.id).Msg("collecting metrics, cpu times")
	} else {
		for idx, v := range ts {
			c.addMetric(&metrics, c.id, fmt.Sprintf("%d%s%s", idx, metricNameSeparator, "user"), "n", v.User)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%d%s%s", idx, metricNameSeparator, "system"), "n", v.System)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%d%s%s", idx, metricNameSeparator, "idle"), "n", v.Idle)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%d%s%s", idx, metricNameSeparator, "nice"), "n", v.Nice)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%d%s%s", idx, metricNameSeparator, "iowait"), "n", v.Iowait)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%d%s%s", idx, metricNameSeparator, "irq"), "n", v.Irq)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%d%s%s", idx, metricNameSeparator, "soft_irq"), "n", v.Softirq)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%d%s%s", idx, metricNameSeparator, "steal"), "n", v.Steal)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%d%s%s", idx, metricNameSeparator, "guest"), "n", v.Guest)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%d%s%s", idx, metricNameSeparator, "guest_nice"), "n", v.GuestNice)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%d%s%s", idx, metricNameSeparator, "stolen"), "n", v.Stolen)
		}
	}

	c.setStatus(metrics, nil)
	return nil
}
