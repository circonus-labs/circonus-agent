// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
	"context"
	"runtime"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/release"
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
		tags.Tag{Category: release.NAME + "-module", Value: c.id},
	}

	var bytesTags tags.Tags
	bytesTags = append(bytesTags, moduleTags...)
	bytesTags = append(bytesTags, tags.Tag{Category: "units", Value: "bytes"})

	var faultsTags tags.Tags
	faultsTags = append(faultsTags, moduleTags...)
	faultsTags = append(faultsTags, tags.Tag{Category: "units", Value: "faults"})

	var hugePagesTags tags.Tags
	hugePagesTags = append(hugePagesTags, moduleTags...)
	hugePagesTags = append(hugePagesTags, tags.Tag{Category: "units", Value: "hugepages"})

	var pagesTags tags.Tags
	pagesTags = append(pagesTags, moduleTags...)
	pagesTags = append(pagesTags, tags.Tag{Category: "units", Value: "pages"})

	var percentTags tags.Tags
	percentTags = append(percentTags, moduleTags...)
	percentTags = append(percentTags, tags.Tag{Category: "units", Value: "percent"})

	metrics := cgm.Metrics{}
	swap, err := mem.SwapMemoryWithContext(context.Background())
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting swap memory metrics")
	} else {
		{
			// units:bytes
			_ = c.addMetric(&metrics, "swap_total", "L", swap.Total, bytesTags)
			_ = c.addMetric(&metrics, "swap_used", "L", swap.Used, bytesTags)
			_ = c.addMetric(&metrics, "swap_free", "L", swap.Free, bytesTags)
			_ = c.addMetric(&metrics, "swap_in", "L", swap.Sin, bytesTags)
			_ = c.addMetric(&metrics, "swap_out", "L", swap.Sout, bytesTags)
		}
		{
			// units:pages
			_ = c.addMetric(&metrics, "swap_in", "L", swap.PgIn, pagesTags)
			_ = c.addMetric(&metrics, "swap_out", "L", swap.PgOut, pagesTags)
		}
		{
			// units:faults
			_ = c.addMetric(&metrics, "pg_fault", "L", swap.PgFault, faultsTags)
		}
		{
			// units:percent
			_ = c.addMetric(&metrics, "swap_used", "n", swap.UsedPercent, percentTags)
		}
	}

	vm, err := mem.VirtualMemory()
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting virtual memory metrics")
	} else {
		{
			// units:bytes
			_ = c.addMetric(&metrics, "total", "L", vm.Total, bytesTags)
			_ = c.addMetric(&metrics, "available", "L", vm.Available, bytesTags)
			_ = c.addMetric(&metrics, "used", "L", vm.Used, bytesTags)
			_ = c.addMetric(&metrics, "free", "L", vm.Free, bytesTags)
		}
		{
			// units:percent
			_ = c.addMetric(&metrics, "used", "n", vm.UsedPercent, percentTags)
		}
		if strings.Contains(runtime.GOOS, "bsd") || runtime.GOOS == "darwin" {
			// OSX / BSD
			_ = c.addMetric(&metrics, "active", "L", vm.Active, bytesTags)
			_ = c.addMetric(&metrics, "inactive", "L", vm.Inactive, bytesTags)
			_ = c.addMetric(&metrics, "wired", "L", vm.Wired, bytesTags)
		}
		if runtime.GOOS == "freebsd" {
			// FreeBSD
			_ = c.addMetric(&metrics, "laundry", "L", vm.Laundry, bytesTags)
		}
		if runtime.GOOS == "linux" {
			// Linux
			{
				// units:bytes -- gopsutil translates the kB in meminfo to bytes
				_ = c.addMetric(&metrics, "buffers", "L", vm.Buffers, bytesTags)
				_ = c.addMetric(&metrics, "cached", "L", vm.Cached, bytesTags)
				_ = c.addMetric(&metrics, "writeback", "L", vm.Writeback, bytesTags)
				_ = c.addMetric(&metrics, "dirty", "L", vm.Dirty, bytesTags)
				_ = c.addMetric(&metrics, "commit_limit", "L", vm.CommitLimit, bytesTags)
				_ = c.addMetric(&metrics, "committed_as", "L", vm.CommittedAS, bytesTags)
				_ = c.addMetric(&metrics, "vm_alloc_total", "L", vm.VMallocTotal, bytesTags)
				_ = c.addMetric(&metrics, "vm_alloc_used", "L", vm.VMallocUsed, bytesTags)
				_ = c.addMetric(&metrics, "vm_alloc_chunk", "L", vm.VMallocChunk, bytesTags)
				_ = c.addMetric(&metrics, "huge_page_size", "L", vm.HugePageSize, bytesTags)
				_ = c.addMetric(&metrics, "writeback_tmp", "L", vm.WritebackTmp, bytesTags)
				_ = c.addMetric(&metrics, "shared", "L", vm.Shared, bytesTags)
				_ = c.addMetric(&metrics, "slab", "L", vm.Slab, bytesTags)
				_ = c.addMetric(&metrics, "slab_reclaimable", "L", vm.SReclaimable, bytesTags)
				_ = c.addMetric(&metrics, "page_tables", "L", vm.PageTables, bytesTags)
				_ = c.addMetric(&metrics, "high_total", "L", vm.HighTotal, bytesTags)
				_ = c.addMetric(&metrics, "high_free", "L", vm.HighFree, bytesTags)
				_ = c.addMetric(&metrics, "low_total", "L", vm.LowTotal, bytesTags)
				_ = c.addMetric(&metrics, "low_free", "L", vm.LowFree, bytesTags)
				_ = c.addMetric(&metrics, "swapfree", "L", vm.SwapFree, bytesTags)
				_ = c.addMetric(&metrics, "swapcached", "L", vm.SwapCached, bytesTags)
				_ = c.addMetric(&metrics, "swaptotal", "L", vm.SwapTotal, bytesTags)
				_ = c.addMetric(&metrics, "mapped", "L", vm.Mapped, bytesTags)
			}
			{
				// units:hugepages
				_ = c.addMetric(&metrics, "huge_pages_total", "L", vm.HugePagesTotal, hugePagesTags)
				_ = c.addMetric(&metrics, "huge_pages_free", "L", vm.HugePagesFree, hugePagesTags)
			}
		}
	}

	c.setStatus(metrics, nil)
	return nil
}
