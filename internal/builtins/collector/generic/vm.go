// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
	"context"
	"math"
	"runtime"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/mem"
)

// VM metrics
type VM struct {
	gencommon
}

// vmOptions defines what elements can be overridden in a config file
type vmOptions struct {
	// common
	ID     string `json:"id" toml:"id" yaml:"id"`
	RunTTL string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewVMCollector creates new psutils collector
func NewVMCollector(cfgBaseName string, parentLogger zerolog.Logger) (collector.Collector, error) {
	c := VM{}
	c.id = NameVM
	c.pkgID = PackageName + "." + c.id
	c.logger = parentLogger.With().Str("id", c.id).Logger()
	c.baseTags = tags.FromList(tags.GetBaseTags())

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

	if opts.RunTTL != "" {
		dur, err := time.ParseDuration(opts.RunTTL)
		if err != nil {
			return nil, errors.Wrapf(err, "%s parsing run_ttl", c.pkgID)
		}
		c.runTTL = dur
	}

	return &c, nil
}

// Collect memory metrics
func (c *VM) Collect(ctx context.Context) error {
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

	tagUnitsBytes := tags.Tag{Category: "units", Value: "bytes"}
	tagUnitsFaults := tags.Tag{Category: "units", Value: "faults"}
	tagUnitsHugePages := tags.Tag{Category: "units", Value: "hugepages"}
	tagUnitsPages := tags.Tag{Category: "units", Value: "pages"}
	tagUnitsPercent := tags.Tag{Category: "units", Value: "percent"}

	metrics := cgm.Metrics{}
	swap, err := mem.SwapMemoryWithContext(context.Background())
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting swap memory metrics")
	} else {
		{ // units:bytes
			tagList := tags.Tags{tagUnitsBytes}
			_ = c.addMetric(&metrics, "swap_total", "L", swap.Total, tagList)
			_ = c.addMetric(&metrics, "swap_used", "L", swap.Used, tagList)
			_ = c.addMetric(&metrics, "swap_free", "L", swap.Free, tagList)
			_ = c.addMetric(&metrics, "swap_in", "L", swap.Sin, tagList)
			_ = c.addMetric(&metrics, "swap_out", "L", swap.Sout, tagList)
		}
		{ // units:pages
			tagList := tags.Tags{tagUnitsPages}
			_ = c.addMetric(&metrics, "swap_in", "L", swap.PgIn, tagList)
			_ = c.addMetric(&metrics, "swap_out", "L", swap.PgOut, tagList)
		}
		{ // units:faults
			tagList := tags.Tags{tagUnitsFaults}
			_ = c.addMetric(&metrics, "pg_fault", "L", swap.PgFault, tagList)
		}
		{ // units:percent
			tagList := tags.Tags{tagUnitsPercent}
			if !math.IsNaN(swap.UsedPercent) {
				_ = c.addMetric(&metrics, "swap_used", "n", swap.UsedPercent, tagList)
			}
		}
	}

	vm, err := mem.VirtualMemory()
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting virtual memory metrics")
		c.setStatus(metrics, nil)
		return nil
	}

	{ // units:bytes
		tagList := tags.Tags{tagUnitsBytes}
		_ = c.addMetric(&metrics, "memory_total", "L", vm.Total, tagList)
		_ = c.addMetric(&metrics, "memory_available", "L", vm.Available, tagList)
		_ = c.addMetric(&metrics, "memory_used", "L", vm.Used, tagList)
		_ = c.addMetric(&metrics, "memory_free", "L", vm.Free, tagList)
	}
	{ // units:percent
		tagList := tags.Tags{tagUnitsPercent}
		if !math.IsNaN(vm.UsedPercent) {
			_ = c.addMetric(&metrics, "memory_used", "n", vm.UsedPercent, tagList)
		}
	}

	if strings.Contains(runtime.GOOS, "bsd") || runtime.GOOS == "darwin" {
		tagList := tags.Tags{tagUnitsBytes}
		_ = c.addMetric(&metrics, "active", "L", vm.Active, tagList)
		_ = c.addMetric(&metrics, "inactive", "L", vm.Inactive, tagList)
		_ = c.addMetric(&metrics, "wired", "L", vm.Wired, tagList)
	}

	if runtime.GOOS == "freebsd" {
		tagList := tags.Tags{tagUnitsBytes}
		_ = c.addMetric(&metrics, "laundry", "L", vm.Laundry, tagList)
	}

	if runtime.GOOS == "linux" {
		{ // units:bytes -- gopsutil translates the kB in meminfo to bytes
			tagList := tags.Tags{tagUnitsBytes}
			_ = c.addMetric(&metrics, "buffers", "L", vm.Buffers, tagList)
			_ = c.addMetric(&metrics, "cached", "L", vm.Cached, tagList)
			_ = c.addMetric(&metrics, "writeback", "L", vm.Writeback, tagList)
			_ = c.addMetric(&metrics, "dirty", "L", vm.Dirty, tagList)
			_ = c.addMetric(&metrics, "commit_limit", "L", vm.CommitLimit, tagList)
			_ = c.addMetric(&metrics, "committed_as", "L", vm.CommittedAS, tagList)
			_ = c.addMetric(&metrics, "vm_alloc_total", "L", vm.VMallocTotal, tagList)
			_ = c.addMetric(&metrics, "vm_alloc_used", "L", vm.VMallocUsed, tagList)
			_ = c.addMetric(&metrics, "vm_alloc_chunk", "L", vm.VMallocChunk, tagList)
			_ = c.addMetric(&metrics, "huge_page_size", "L", vm.HugePageSize, tagList)
			_ = c.addMetric(&metrics, "writeback_tmp", "L", vm.WritebackTmp, tagList)
			_ = c.addMetric(&metrics, "shared", "L", vm.Shared, tagList)
			_ = c.addMetric(&metrics, "slab", "L", vm.Slab, tagList)
			_ = c.addMetric(&metrics, "slab_reclaimable", "L", vm.SReclaimable, tagList)
			_ = c.addMetric(&metrics, "page_tables", "L", vm.PageTables, tagList)
			_ = c.addMetric(&metrics, "high_total", "L", vm.HighTotal, tagList)
			_ = c.addMetric(&metrics, "high_free", "L", vm.HighFree, tagList)
			_ = c.addMetric(&metrics, "low_total", "L", vm.LowTotal, tagList)
			_ = c.addMetric(&metrics, "low_free", "L", vm.LowFree, tagList)
			_ = c.addMetric(&metrics, "swapfree", "L", vm.SwapFree, tagList)
			_ = c.addMetric(&metrics, "swapcached", "L", vm.SwapCached, tagList)
			_ = c.addMetric(&metrics, "swaptotal", "L", vm.SwapTotal, tagList)
			_ = c.addMetric(&metrics, "mapped", "L", vm.Mapped, tagList)
		}

		{ // units:hugepages
			tagList := tags.Tags{tagUnitsHugePages}
			_ = c.addMetric(&metrics, "huge_pages_total", "L", vm.HugePagesTotal, tagList)
			_ = c.addMetric(&metrics, "huge_pages_free", "L", vm.HugePagesFree, tagList)
		}
	}

	c.setStatus(metrics, nil)
	return nil
}
