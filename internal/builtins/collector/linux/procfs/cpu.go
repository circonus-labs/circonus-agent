// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build linux

package procfs

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
)

// CPU metrics from the Linux ProcFS
type CPU struct {
	common
	numCPU        float64               // number of cpus
	clockNorm     float64               // cpu clock normalized to 100Hz tick rate
	reportAllCPUs bool                  // OPT report all cpus (vs just total) may be overridden in config file
	lastRunValues map[string]lastValues // values from last run
}

// cpuOptions defines what elements can be overridden in a config file
type cpuOptions struct {
	// common
	ID         string `json:"id" toml:"id" yaml:"id"`
	ProcFSPath string `json:"procfs_path" toml:"procfs_path" yaml:"procfs_path"`
	RunTTL     string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`

	// collector specific
	ClockHZ string `json:"clock_hz" toml:"clock_hz" yaml:"clock_hz"`
	AllCPU  string `json:"report_all_cpus" toml:"report_all_cpus" yaml:"report_all_cpus"`
}

type lastValues struct {
	all  float64
	busy float64
}

// NewCPUCollector creates new procfs cpu collector
func NewCPUCollector(cfgBaseName, procFSPath string) (collector.Collector, error) {
	procFile := "stat"

	c := CPU{
		common:        newCommon(NameCPU, procFSPath, procFile, tags.FromList(tags.GetBaseTags())),
		lastRunValues: make(map[string]lastValues),
	}

	c.numCPU = float64(runtime.NumCPU())
	clockHZ := float64(100)
	c.clockNorm = clockHZ / 100
	c.reportAllCPUs = false

	if cfgBaseName == "" {
		if _, err := os.Stat(c.file); os.IsNotExist(err) {
			return nil, errors.Wrap(err, c.pkgID)
		}

		return &c, nil
	}

	var opts cpuOptions
	err := config.LoadConfigFile(cfgBaseName, &opts)
	if err != nil {
		if !strings.Contains(err.Error(), "no config found matching") {
			c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
			return nil, errors.Wrapf(err, "%s config", c.pkgID)
		}
	} else {
		c.logger.Debug().Interface("config", opts).Msg("loaded config")
	}

	if opts.ClockHZ != "" {
		v, err := strconv.ParseFloat(opts.ClockHZ, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "%s parsing clock_hz", c.pkgID)
		}
		clockHZ = v
		c.clockNorm = clockHZ / 100
	}

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

	if opts.ProcFSPath != "" {
		c.procFSPath = opts.ProcFSPath
		c.file = filepath.Join(c.procFSPath, procFile)
	}

	if opts.RunTTL != "" {
		dur, err := time.ParseDuration(opts.RunTTL)
		if err != nil {
			return nil, errors.Wrapf(err, "%s parsing run_ttl", c.pkgID)
		}
		c.runTTL = dur
	}

	if _, err := os.Stat(c.file); os.IsNotExist(err) {
		return nil, errors.Wrap(err, c.pkgID)
	}

	return &c, nil
}

// Collect metrics from the procfs resource
func (c *CPU) Collect(ctx context.Context) error {
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

	tagUnitsCentiseconds := tags.Tag{Category: "units", Value: "centiseconds"} // aka jiffies
	tagUnitsPercent := tags.Tag{Category: "units", Value: "percent"}

	lines, err := c.readFile(c.file)
	if err != nil {
		c.setStatus(metrics, err)
		return errors.Wrap(err, c.pkgID)
	}

	_ = c.addMetric(&metrics, "", "num_cpu", "I", runtime.NumCPU(), tags.Tags{})

	for _, line := range lines {
		fields := strings.Fields(line)

		switch fields[0] {
		case "processes":
			v, err := strconv.ParseInt(fields[1], 10, 64)
			if err == nil {
				_ = c.addMetric(&metrics, "", "processes", "I", v, tags.Tags{})
			} else {
				c.logger.Warn().Err(err).Str("line", line).Str("value", fields[1]).Msg("parsing int")
			}
		case "procs_running":
			v, err := strconv.ParseInt(fields[1], 10, 64)
			if err == nil {
				_ = c.addMetric(&metrics, "", "procs_runnable", "I", v, tags.Tags{})
			} else {
				c.logger.Warn().Err(err).Str("line", line).Str("value", fields[1]).Msg("parsing int")
			}
		case "procs_blocked":
			v, err := strconv.ParseInt(fields[1], 10, 64)
			if err == nil {
				_ = c.addMetric(&metrics, "", "procs_blocked", "I", v, tags.Tags{})
			} else {
				c.logger.Warn().Err(err).Str("line", line).Str("value", fields[1]).Msg("parsing int")
			}
		}

		if !strings.HasPrefix(fields[0], c.id) {
			continue
		}

		if fields[0] != c.id && !c.reportAllCPUs {
			continue
		}

		id, cpuMetrics, err := c.parseCPU(fields)
		if err != nil {
			c.setStatus(metrics, err)
			return errors.Wrapf(err, "%s parsing %s", c.pkgID, fields[0])
		}

		for mn, mv := range *cpuMetrics {
			var tagList tags.Tags

			if id != "" {
				tagList = append(tagList, tags.Tag{Category: "cpu", Value: id})
			}

			if mn == "cpu_used" {
				tagList = append(tagList, tagUnitsPercent)
			} else {
				tagList = append(tagList, tagUnitsCentiseconds)
			}

			_ = c.addMetric(&metrics, "", mn, mv.Type, mv.Value, tagList)
		}

	}

	c.setStatus(metrics, nil)
	return nil
}

func (c *CPU) parseCPU(fields []string) (string, *cgm.Metrics, error) {
	var numCPU float64
	var cpuID string

	if fields[0] == "cpu" {
		numCPU = c.numCPU // aggregate cpu metrics
	} else {
		numCPU = 1 // individual cpu metrics
		cpuID = strings.Replace(fields[0], "cpu", "", 1)
	}

	metricType := "n" // resmon double

	busy := float64(0)

	userNormal, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return cpuID, nil, err
	}
	busy += userNormal

	userNice, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return cpuID, nil, err
	}
	busy += userNice

	sys, err := strconv.ParseFloat(fields[3], 64)
	if err != nil {
		return cpuID, nil, err
	}
	busy += sys

	idleNormal, err := strconv.ParseFloat(fields[4], 64)
	if err != nil {
		return cpuID, nil, err
	}

	waitIO, err := strconv.ParseFloat(fields[5], 64)
	if err != nil {
		return cpuID, nil, err
	}
	busy += waitIO

	irq, err := strconv.ParseFloat(fields[6], 64)
	if err != nil {
		return cpuID, nil, err
	}
	busy += irq

	softIRQ, err := strconv.ParseFloat(fields[7], 64)
	if err != nil {
		return cpuID, nil, err
	}
	busy += softIRQ

	steal := float64(0)
	if len(fields) > 8 {
		v, err := strconv.ParseFloat(fields[8], 64)
		if err != nil {
			return cpuID, nil, err
		}
		steal = v
		busy += steal
	}

	guest := float64(0)
	if len(fields) > 9 {
		v, err := strconv.ParseFloat(fields[9], 64)
		if err != nil {
			return cpuID, nil, err
		}
		guest = v
		busy += guest
	}

	guestNice := float64(0)
	if len(fields) > 10 {
		v, err := strconv.ParseFloat(fields[10], 64)
		if err != nil {
			return cpuID, nil, err
		}
		guestNice = v
		busy += guestNice
	}

	metrics := cgm.Metrics{
		"cpu_user":       cgm.Metric{Type: metricType, Value: (userNormal / numCPU) / c.clockNorm},
		"cpu_system":     cgm.Metric{Type: metricType, Value: (sys / numCPU) / c.clockNorm},
		"cpu_idle":       cgm.Metric{Type: metricType, Value: (idleNormal / numCPU) / c.clockNorm},
		"cpu_nice":       cgm.Metric{Type: metricType, Value: (userNice / numCPU) / c.clockNorm},
		"cpu_iowait":     cgm.Metric{Type: metricType, Value: (waitIO / numCPU) / c.clockNorm},
		"cpu_irq":        cgm.Metric{Type: metricType, Value: (irq / numCPU) / c.clockNorm},
		"cpu_soft_irq":   cgm.Metric{Type: metricType, Value: (softIRQ / numCPU) / c.clockNorm},
		"cpu_steal":      cgm.Metric{Type: metricType, Value: (steal / numCPU) / c.clockNorm},
		"cpu_guest":      cgm.Metric{Type: metricType, Value: (guest / numCPU) / c.clockNorm},
		"cpu_guest_nice": cgm.Metric{Type: metricType, Value: (guestNice / numCPU) / c.clockNorm},
	}

	all := float64(busy + idleNormal)
	if lrv, ok := c.lastRunValues[fields[0]]; ok {
		used := ((busy - lrv.busy) / (all - lrv.all)) * 100
		metrics["cpu_used"] = cgm.Metric{Type: metricType, Value: used}
	}
	c.lastRunValues[fields[0]] = lastValues{all: all, busy: busy}

	return cpuID, &metrics, nil
}
