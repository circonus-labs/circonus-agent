// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build linux

package procfs

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// VM metrics from the Linux ProcFS
type VM struct {
	common
}

// vmOptions defines what elements can be overridden in a config file
type vmOptions struct {
	// common
	ID         string `json:"id" toml:"id" yaml:"id"`
	ProcFSPath string `json:"procfs_path" toml:"procfs_path" yaml:"procfs_path"`
	RunTTL     string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewVMCollector creates new procfs vm collector
func NewVMCollector(cfgBaseName, procFSPath string) (collector.Collector, error) {
	procFile := "meminfo"

	c := VM{}
	c.id = NameVM
	c.pkgID = PKG_NAME + "." + c.id
	c.logger = log.With().Str("pkg", PKG_NAME).Str("id", c.id).Logger()
	c.procFSPath = procFSPath
	c.file = filepath.Join(c.procFSPath, procFile)
	c.baseTags = tags.FromList(tags.GetBaseTags())

	if cfgBaseName == "" {
		if _, err := os.Stat(c.file); err != nil {
			return nil, errors.Wrap(err, c.pkgID)
		}
		return &c, nil
	}

	var opts vmOptions
	err := config.LoadConfigFile(cfgBaseName, &opts)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, errors.Wrapf(err, "%s config", c.pkgID)
	}

	c.logger.Debug().Str("base", cfgBaseName).Interface("config", opts).Msg("loaded config")

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
func (c *VM) Collect() error {
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

	if err := c.parseMemstats(&metrics); err != nil {
		c.setStatus(metrics, err)
		return errors.Wrap(err, c.pkgID)
	}

	if err := c.parseVMstats(&metrics); err != nil {
		c.setStatus(metrics, err)
		return errors.Wrap(err, c.pkgID)
	}

	c.setStatus(metrics, nil)
	return nil
}

func (c *VM) parseMemstats(metrics *cgm.Metrics) error {
	lines, err := c.readFile(c.file)
	if err != nil {
		return errors.Wrapf(err, "parsing %s", c.file)
	}

	stats := make(map[string]uint64)

	for _, l := range lines {
		line := strings.TrimSpace(string(l))
		fields := strings.Fields(line)

		if len(fields) < 2 {
			continue
		}

		name := strings.Replace(fields[0], ":", "", -1)
		vs := strings.TrimSpace(fields[1])
		units := ""
		if len(fields) > 2 {
			units = fields[2]
		}

		v, err := strconv.ParseUint(vs, 10, 64)
		if err != nil {
			c.logger.Warn().Err(err).Msg("parsing field " + name)
			continue
		}

		if strings.ToLower(units) == "kb" {
			v *= uint64(1024)
		}

		stats[name] = v
	}

	var memTotal, memFree, memCached, memBuffers, memSReclaimable, memShared, swapTotal, swapFree uint64
	for metricName, mval := range stats {
		// pfx := c.id + metricNameSeparator + "meminfo"
		// mname := metricName
		// mtype := "L"
		switch metricName {
		case "MemTotal":
			memTotal = mval
		case "MemFree":
			memFree = mval
		case "SwapTotal":
			swapTotal = mval
		case "SwapFree":
			swapFree = mval
		case "SReclaimable":
			memSReclaimable = mval
		case "Shmem":
			memShared = mval
		case "Buffers":
			memBuffers = mval
		case "Cached":
			memCached = mval
		}
		// c.addMetric(metrics, pfx, mname, mtype, mval)
	}

	// `htop` based calculations
	htUsed := memTotal - memFree
	htCached := memCached + (memSReclaimable - memShared)
	htBuffers := memBuffers
	htUsedTotal := htUsed - (htBuffers + htCached)
	htFreeTotal := memFree + htBuffers + htCached

	// old `free` based calculations carried over from vm.sh
	// memFreeTotal := memFree + memBuffers + memCached
	// memUsed := memTotal - memFreeTotal
	memFreeTotal := htFreeTotal
	memUsed := htUsedTotal

	memFreePct := (float64(memFreeTotal) / float64(memTotal)) * 100
	memUsedPct := (float64(memUsed) / float64(memTotal)) * 100

	swapUsed := swapTotal - swapFree
	swapFreePct := 0.0
	swapUsedPct := 0.0
	if swapTotal > 0 {
		swapFreePct = (float64(swapFree) / float64(swapTotal)) * 100
		swapUsedPct = (float64(swapUsed) / float64(swapTotal)) * 100
	}

	tagUnitsBytes := tags.Tag{Category: "units", Value: "bytes"}
	tagUnitsPercent := tags.Tag{Category: "units", Value: "percent"}

	// pfx := c.id + metricNameSeparator + "memory"
	_ = c.addMetric(metrics, "", "memory_total", "L", memTotal, tags.Tags{tagUnitsBytes})
	_ = c.addMetric(metrics, "", "memory_free", "L", memFreeTotal, tags.Tags{tagUnitsBytes})
	_ = c.addMetric(metrics, "", "memory_free", "n", memFreePct, tags.Tags{tagUnitsPercent})
	_ = c.addMetric(metrics, "", "memory_used", "n", memUsedPct, tags.Tags{tagUnitsPercent})
	_ = c.addMetric(metrics, "", "memory_used", "L", memUsed, tags.Tags{tagUnitsBytes})
	_ = c.addMetric(metrics, "", "buffers", "L", memBuffers, tags.Tags{tagUnitsBytes})
	_ = c.addMetric(metrics, "", "cached", "L", memCached, tags.Tags{tagUnitsBytes})
	_ = c.addMetric(metrics, "", "shared", "L", memShared, tags.Tags{tagUnitsBytes})
	_ = c.addMetric(metrics, "", "cached", "L", memCached, tags.Tags{tagUnitsBytes})
	_ = c.addMetric(metrics, "", "slab_reclaimable", "L", memSReclaimable, tags.Tags{tagUnitsBytes})

	// pfx = c.id + metricNameSeparator + "swap"
	_ = c.addMetric(metrics, "", "swap_total", "L", swapTotal, tags.Tags{tagUnitsBytes})
	_ = c.addMetric(metrics, "", "swap_free", "L", swapTotal-swapUsed, tags.Tags{tagUnitsBytes})
	_ = c.addMetric(metrics, "", "swap_free", "n", swapFreePct, tags.Tags{tagUnitsPercent})
	_ = c.addMetric(metrics, "", "swap_used", "L", swapUsed, tags.Tags{tagUnitsBytes})
	_ = c.addMetric(metrics, "", "swap_used", "n", swapUsedPct*100, tags.Tags{tagUnitsPercent})

	return nil
}

func (c *VM) parseVMstats(metrics *cgm.Metrics) error {
	file := strings.Replace(c.file, "meminfo", "vmstat", -1)
	lines, err := c.readFile(file)
	if err != nil {
		return errors.Wrapf(err, "parsing %s", file)
	}

	var pgFaults, pgMajorFaults, pgScan, pgSwap uint64

	for _, l := range lines {
		line := strings.TrimSpace(string(l))
		fields := strings.Fields(line)

		if len(fields) != 2 {
			continue
		}

		switch {
		case fields[0] == "pgfault":
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				c.logger.Warn().Err(err).Msg("parsing field " + fields[0])
				continue
			}
			pgFaults = v

		case fields[0] == "pgmajfault":
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				c.logger.Warn().Err(err).Msg("parsing field " + fields[0])
				continue
			}
			pgMajorFaults = v

		case strings.HasPrefix(fields[0], "pswp"):
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				c.logger.Warn().Err(err).Msg("parsing field " + fields[0])
				continue
			}
			pgSwap = v

		case strings.HasPrefix(fields[0], "pgscan"):
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				c.logger.Warn().Err(err).Msg("parsing field " + fields[0])
				continue
			}
			pgScan += v

		default:
			// ignore
		}
	}

	metricType := "L"
	tagUnitsFaults := tags.Tag{Category: "units", Value: "faults"}
	tagUnitsScans := tags.Tag{Category: "units", Value: "scans"}
	tagUnitsSwaps := tags.Tag{Category: "units", Value: "swaps"}
	_ = c.addMetric(metrics, "", "pg_fault", metricType, pgFaults, tags.Tags{tagUnitsFaults})
	_ = c.addMetric(metrics, "", "pg_fault_major", metricType, pgMajorFaults, tags.Tags{tagUnitsFaults})
	_ = c.addMetric(metrics, "", "pg_fault__minor", metricType, pgFaults-pgMajorFaults, tags.Tags{tagUnitsFaults})
	_ = c.addMetric(metrics, "", "pg_swap", "L", pgSwap, tags.Tags{tagUnitsSwaps})
	_ = c.addMetric(metrics, "", "pg_scan", metricType, pgScan, tags.Tags{tagUnitsScans})

	return nil
}
