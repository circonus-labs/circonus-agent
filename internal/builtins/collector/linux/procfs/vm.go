// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build linux

package procfs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
)

// VM metrics from the Linux ProcFS.
type VM struct {
	common
}

// vmOptions defines what elements can be overridden in a config file.
type vmOptions struct {
	// common
	ID         string `json:"id" toml:"id" yaml:"id"`
	ProcFSPath string `json:"procfs_path" toml:"procfs_path" yaml:"procfs_path"`
	RunTTL     string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewVMCollector creates new procfs vm collector.
func NewVMCollector(cfgBaseName, procFSPath string) (collector.Collector, error) {
	procFile := "meminfo"

	c := VM{
		common: newCommon(NameVM, procFSPath, procFile, tags.FromList(tags.GetBaseTags())),
	}

	if cfgBaseName == "" {
		if _, err := os.Stat(c.file); err != nil {
			return nil, fmt.Errorf("%s procfile: %w", c.pkgID, err)
		}
		return &c, nil
	}

	var opts vmOptions
	err := config.LoadConfigFile(cfgBaseName, &opts)
	if err != nil {
		if !strings.Contains(err.Error(), "no config found matching") {
			c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
			return nil, fmt.Errorf("%s config: %w", c.pkgID, err)
		}
	} else {
		c.logger.Debug().Str("base", cfgBaseName).Interface("config", opts).Msg("loaded config")
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
			return nil, fmt.Errorf("%s parsing run_ttl: %w", c.pkgID, err)
		}
		c.runTTL = dur
	}

	if _, err := os.Stat(c.file); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s procfile: %w", c.pkgID, err)
	}

	return &c, nil
}

// Collect metrics from the procfs resource.
func (c *VM) Collect(ctx context.Context) error {
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
		return fmt.Errorf("%s parseMemstats: %w", c.pkgID, err)
	}

	if err := c.parseVMstats(&metrics); err != nil {
		c.setStatus(metrics, err)
		return fmt.Errorf("%s parseVMstats: %w", c.pkgID, err)
	}

	c.setStatus(metrics, nil)
	return nil
}

func (c *VM) parseMemstats(metrics *cgm.Metrics) error {
	lines, err := c.readFile(c.file)
	if err != nil {
		return fmt.Errorf("%s read file: %w", c.pkgID, err)
	}

	var memTotal, memFree, memCached, memBuffers, memSReclaimable, memShared, swapTotal, swapFree uint64
	tagUnitsBytes := tags.Tag{Category: "units", Value: "bytes"}
	tagUnitsPercent := tags.Tag{Category: "units", Value: "percent"}
	tagUnitsHugePages := tags.Tag{Category: "units", Value: "hugepages"}

	for _, l := range lines {
		line := strings.TrimSpace(l)
		fields := strings.Fields(line)

		if len(fields) < 2 {
			continue
		}

		name := strings.ReplaceAll(fields[0], ":", "")
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

		switch name {
		case "MemTotal":
			memTotal = v
		case "MemFree":
			memFree = v
		case "MemAvailable":
			_ = c.addMetric(metrics, "", "memory_available", "L", v, tags.Tags{tagUnitsBytes})
		case "Buffers":
			memBuffers = v
		case "Cached":
			memCached = v
		case "Active":
			_ = c.addMetric(metrics, "", "active", "L", v, tags.Tags{tagUnitsBytes})
		case "Inactive":
			_ = c.addMetric(metrics, "", "inactive", "L", v, tags.Tags{tagUnitsBytes})
		case "Active(file)":
			_ = c.addMetric(metrics, "", "active_file", "L", v, tags.Tags{tagUnitsBytes})
		case "Inactive(file)":
			_ = c.addMetric(metrics, "", "inactive_file", "L", v, tags.Tags{tagUnitsBytes})
		case "Writeback":
			_ = c.addMetric(metrics, "", "writeback", "L", v, tags.Tags{tagUnitsBytes})
		case "WritebackTmp":
			_ = c.addMetric(metrics, "", "writeback_tmp", "L", v, tags.Tags{tagUnitsBytes})
		case "Dirty":
			_ = c.addMetric(metrics, "", "dirty", "L", v, tags.Tags{tagUnitsBytes})
		case "Shmem":
			memShared = v
		case "Slab":
			_ = c.addMetric(metrics, "", "slab", "L", v, tags.Tags{tagUnitsBytes})
		case "SReclaimable":
			memSReclaimable = v
		case "PageTables":
			_ = c.addMetric(metrics, "", "page_tables", "L", v, tags.Tags{tagUnitsBytes})
		case "SwapCached":
			_ = c.addMetric(metrics, "", "swap_cached", "L", v, tags.Tags{tagUnitsBytes})
		case "CommitLimit":
			_ = c.addMetric(metrics, "", "commit_limit", "L", v, tags.Tags{tagUnitsBytes})
		case "Committed_AS":
			_ = c.addMetric(metrics, "", "committed_as", "L", v, tags.Tags{tagUnitsBytes})
		case "HighTotal":
			_ = c.addMetric(metrics, "", "high_total", "L", v, tags.Tags{tagUnitsBytes})
		case "HighFree":
			_ = c.addMetric(metrics, "", "high_free", "L", v, tags.Tags{tagUnitsBytes})
		case "LowTotal":
			_ = c.addMetric(metrics, "", "low_total", "L", v, tags.Tags{tagUnitsBytes})
		case "LowFree":
			_ = c.addMetric(metrics, "", "low_free", "L", v, tags.Tags{tagUnitsBytes})
		case "SwapTotal":
			swapTotal = v
		case "SwapFree":
			swapFree = v
		case "Mapped":
			_ = c.addMetric(metrics, "", "mapped", "L", v, tags.Tags{tagUnitsBytes})
		case "VmallocTotal":
			_ = c.addMetric(metrics, "", "vm_alloc_total", "L", v, tags.Tags{tagUnitsBytes})
		case "VmallocUsed":
			_ = c.addMetric(metrics, "", "vm_alloc_used", "L", v, tags.Tags{tagUnitsBytes})
		case "VmallocChunk":
			_ = c.addMetric(metrics, "", "vm_alloc_chunk", "L", v, tags.Tags{tagUnitsBytes})
		case "HugePages_Total":
			_ = c.addMetric(metrics, "", "huge_pages_total", "L", v, tags.Tags{tagUnitsHugePages})
		case "HugePages_Free":
			_ = c.addMetric(metrics, "", "huge_pages_free", "L", v, tags.Tags{tagUnitsHugePages})
		case "Hugepagesize":
			_ = c.addMetric(metrics, "", "huge_page_size", "L", v, tags.Tags{tagUnitsBytes})
		}
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

	_ = c.addMetric(metrics, "", "memory_total", "L", memTotal, tags.Tags{tagUnitsBytes})
	_ = c.addMetric(metrics, "", "memory_free", "L", memFreeTotal, tags.Tags{tagUnitsBytes})
	_ = c.addMetric(metrics, "", "memory_free", "n", memFreePct, tags.Tags{tagUnitsPercent})
	_ = c.addMetric(metrics, "", "memory_used", "n", memUsedPct, tags.Tags{tagUnitsPercent})
	_ = c.addMetric(metrics, "", "memory_used", "L", memUsed, tags.Tags{tagUnitsBytes})
	_ = c.addMetric(metrics, "", "buffers", "L", memBuffers, tags.Tags{tagUnitsBytes})
	_ = c.addMetric(metrics, "", "cached", "L", memCached, tags.Tags{tagUnitsBytes})
	_ = c.addMetric(metrics, "", "shared", "L", memShared, tags.Tags{tagUnitsBytes})
	_ = c.addMetric(metrics, "", "slab_reclaimable", "L", memSReclaimable, tags.Tags{tagUnitsBytes})

	_ = c.addMetric(metrics, "", "swap_total", "L", swapTotal, tags.Tags{tagUnitsBytes})
	_ = c.addMetric(metrics, "", "swap_free", "L", swapTotal-swapUsed, tags.Tags{tagUnitsBytes})
	_ = c.addMetric(metrics, "", "swap_free", "n", swapFreePct, tags.Tags{tagUnitsPercent})
	_ = c.addMetric(metrics, "", "swap_used", "L", swapUsed, tags.Tags{tagUnitsBytes})
	_ = c.addMetric(metrics, "", "swap_used", "n", swapUsedPct*100, tags.Tags{tagUnitsPercent})

	return nil
}

func (c *VM) parseVMstats(metrics *cgm.Metrics) error {
	file := strings.ReplaceAll(c.file, "meminfo", "vmstat")
	lines, err := c.readFile(file)
	if err != nil {
		return fmt.Errorf("%s read file: %w", c.pkgID, err)
	}

	var pgFaults, pgMajorFaults, pgScan, pgSwap uint64

	for _, l := range lines {
		line := strings.TrimSpace(l)
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
