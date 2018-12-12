// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/mem"
)

// VM metrics
type VM struct {
	common
}

// vmOptions defines what elements can be overridden in a config file
type vmOptions struct {
	// common
	ID                   string   `json:"id" toml:"id" yaml:"id"`
	MetricsEnabled       []string `json:"metrics_enabled" toml:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsDisabled      []string `json:"metrics_disabled" toml:"metrics_disabled" yaml:"metrics_disabled"`
	MetricsDefaultStatus string   `json:"metrics_default_status" toml:"metrics_default_status" toml:"metrics_default_status"`
	RunTTL               string   `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewVMCollector creates new psutils collector
func NewVMCollector(cfgBaseName string) (collector.Collector, error) {
	c := VM{}
	c.id = VM_NAME
	c.pkgID = LOG_PREFIX + c.id
	c.logger = log.With().Str("pkg", c.pkgID).Logger()
	c.metricStatus = map[string]bool{}
	c.metricDefaultActive = true
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

	metrics := cgm.Metrics{}
	swap, err := mem.SwapMemory()
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting swap memory metrics")
	} else {
		c.addMetric(&metrics, c.id, "swap_total", "L", swap.Total)
		c.addMetric(&metrics, c.id, "swap_used", "L", swap.Used)
		c.addMetric(&metrics, c.id, "swap_free", "L", swap.Free)
		c.addMetric(&metrics, c.id, "swap_used_pct", "n", swap.UsedPercent)
		c.addMetric(&metrics, c.id, "swap_in", "L", swap.Sin)
		c.addMetric(&metrics, c.id, "swap_out", "L", swap.Sout)
	}

	vm, err := mem.VirtualMemory()
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting virtual memory metrics")
	} else {
		c.addMetric(&metrics, c.id, "total", "L", vm.Total)
		c.addMetric(&metrics, c.id, "available", "L", vm.Available)
		c.addMetric(&metrics, c.id, "used", "L", vm.Used)
		c.addMetric(&metrics, c.id, "used_pct", "n", vm.UsedPercent)
		c.addMetric(&metrics, c.id, "free", "L", vm.Free)
		if strings.Contains(runtime.GOOS, "bsd") || runtime.GOOS == "darwin" {
			// OSX / BSD
			c.addMetric(&metrics, c.id, "active", "L", vm.Active)
			c.addMetric(&metrics, c.id, "inactive", "L", vm.Inactive)
			c.addMetric(&metrics, c.id, "wired", "L", vm.Wired)
		}
		if runtime.GOOS == "freebsd" {
			// FreeBSD
			c.addMetric(&metrics, c.id, "laundry", "L", vm.Laundry)
		}
		if runtime.GOOS == "linux" {
			// Linux
			c.addMetric(&metrics, c.id, "buffers", "L", vm.Buffers)
			c.addMetric(&metrics, c.id, "cached", "L", vm.Cached)
			c.addMetric(&metrics, c.id, "writeback", "L", vm.Writeback)
			c.addMetric(&metrics, c.id, "dirty", "L", vm.Dirty)
			c.addMetric(&metrics, c.id, "writeback_tmp", "L", vm.WritebackTmp)
			c.addMetric(&metrics, c.id, "shared", "L", vm.Shared)
			c.addMetric(&metrics, c.id, "slab", "L", vm.Slab)
			c.addMetric(&metrics, c.id, "page_tables", "L", vm.PageTables)
			c.addMetric(&metrics, c.id, "commit_limit", "L", vm.CommitLimit)
			c.addMetric(&metrics, c.id, "committed_as", "L", vm.CommittedAS)
			c.addMetric(&metrics, c.id, "high_total", "L", vm.HighTotal)
			c.addMetric(&metrics, c.id, "high_free", "L", vm.HighFree)
			c.addMetric(&metrics, c.id, "low_total", "L", vm.LowTotal)
			c.addMetric(&metrics, c.id, "low_free", "L", vm.LowFree)
			c.addMetric(&metrics, c.id, "swapfree", "L", vm.SwapFree)
			c.addMetric(&metrics, c.id, "swapcached", "L", vm.SwapCached)
			c.addMetric(&metrics, c.id, "swaptotal", "L", vm.SwapTotal)
			c.addMetric(&metrics, c.id, "mapped", "L", vm.Mapped)
			c.addMetric(&metrics, c.id, "vm_alloc_total", "L", vm.VMallocTotal)
			c.addMetric(&metrics, c.id, "vm_alloc_used", "L", vm.VMallocUsed)
			c.addMetric(&metrics, c.id, "vm_alloc_chunk", "L", vm.VMallocChunk)
			c.addMetric(&metrics, c.id, "huge_pages_total", "L", vm.HugePagesTotal)
			c.addMetric(&metrics, c.id, "huge_pages_free", "L", vm.HugePagesFree)
			c.addMetric(&metrics, c.id, "huge_page_size", "L", vm.HugePageSize)
		}
	}

	c.setStatus(metrics, nil)
	return nil
}
