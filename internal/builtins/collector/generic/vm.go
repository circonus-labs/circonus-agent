// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
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
func (c *VM) Collect() error {
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
		tags.Tag{Category: "module", Value: c.id},
	}

	metrics := cgm.Metrics{}
	swap, err := mem.SwapMemory()
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting swap memory metrics")
	} else {
		_ = c.addMetric(&metrics, "swap_total", "L", swap.Total, moduleTags)
		_ = c.addMetric(&metrics, "swap_used", "L", swap.Used, moduleTags)
		_ = c.addMetric(&metrics, "swap_free", "L", swap.Free, moduleTags)
		_ = c.addMetric(&metrics, "swap_used_pct", "n", swap.UsedPercent, moduleTags)
		_ = c.addMetric(&metrics, "swap_in", "L", swap.Sin, moduleTags)
		_ = c.addMetric(&metrics, "swap_out", "L", swap.Sout, moduleTags)
	}

	vm, err := mem.VirtualMemory()
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting virtual memory metrics")
	} else {
		_ = c.addMetric(&metrics, "total", "L", vm.Total, moduleTags)
		_ = c.addMetric(&metrics, "available", "L", vm.Available, moduleTags)
		_ = c.addMetric(&metrics, "used", "L", vm.Used, moduleTags)
		_ = c.addMetric(&metrics, "used_pct", "n", vm.UsedPercent, moduleTags)
		_ = c.addMetric(&metrics, "free", "L", vm.Free, moduleTags)
		if strings.Contains(runtime.GOOS, "bsd") || runtime.GOOS == "darwin" {
			// OSX / BSD
			_ = c.addMetric(&metrics, "active", "L", vm.Active, moduleTags)
			_ = c.addMetric(&metrics, "inactive", "L", vm.Inactive, moduleTags)
			_ = c.addMetric(&metrics, "wired", "L", vm.Wired, moduleTags)
		}
		if runtime.GOOS == "freebsd" {
			// FreeBSD
			_ = c.addMetric(&metrics, "laundry", "L", vm.Laundry, moduleTags)
		}
		if runtime.GOOS == "linux" {
			// Linux
			_ = c.addMetric(&metrics, "buffers", "L", vm.Buffers, moduleTags)
			_ = c.addMetric(&metrics, "cached", "L", vm.Cached, moduleTags)
			_ = c.addMetric(&metrics, "writeback", "L", vm.Writeback, moduleTags)
			_ = c.addMetric(&metrics, "dirty", "L", vm.Dirty, moduleTags)
			_ = c.addMetric(&metrics, "writeback_tmp", "L", vm.WritebackTmp, moduleTags)
			_ = c.addMetric(&metrics, "shared", "L", vm.Shared, moduleTags)
			_ = c.addMetric(&metrics, "slab", "L", vm.Slab, moduleTags)
			_ = c.addMetric(&metrics, "page_tables", "L", vm.PageTables, moduleTags)
			_ = c.addMetric(&metrics, "commit_limit", "L", vm.CommitLimit, moduleTags)
			_ = c.addMetric(&metrics, "committed_as", "L", vm.CommittedAS, moduleTags)
			_ = c.addMetric(&metrics, "high_total", "L", vm.HighTotal, moduleTags)
			_ = c.addMetric(&metrics, "high_free", "L", vm.HighFree, moduleTags)
			_ = c.addMetric(&metrics, "low_total", "L", vm.LowTotal, moduleTags)
			_ = c.addMetric(&metrics, "low_free", "L", vm.LowFree, moduleTags)
			_ = c.addMetric(&metrics, "swapfree", "L", vm.SwapFree, moduleTags)
			_ = c.addMetric(&metrics, "swapcached", "L", vm.SwapCached, moduleTags)
			_ = c.addMetric(&metrics, "swaptotal", "L", vm.SwapTotal, moduleTags)
			_ = c.addMetric(&metrics, "mapped", "L", vm.Mapped, moduleTags)
			_ = c.addMetric(&metrics, "vm_alloc_total", "L", vm.VMallocTotal, moduleTags)
			_ = c.addMetric(&metrics, "vm_alloc_used", "L", vm.VMallocUsed, moduleTags)
			_ = c.addMetric(&metrics, "vm_alloc_chunk", "L", vm.VMallocChunk, moduleTags)
			_ = c.addMetric(&metrics, "huge_pages_total", "L", vm.HugePagesTotal, moduleTags)
			_ = c.addMetric(&metrics, "huge_pages_free", "L", vm.HugePagesFree, moduleTags)
			_ = c.addMetric(&metrics, "huge_page_size", "L", vm.HugePageSize, moduleTags)
		}
	}

	c.setStatus(metrics, nil)
	return nil
}
