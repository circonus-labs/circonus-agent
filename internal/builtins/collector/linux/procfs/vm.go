// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package procfs

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	cgm "github.com/circonus-labs/circonus-gometrics"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// VM metrics from the Linux ProcFS
type VM struct {
	pfscommon
}

// vmOptions defines what elements can be overriden in a config file
type vmOptions struct {
	// common
	ID                   string   `json:"id" toml:"id" yaml:"id"`
	ProcFSPath           string   `json:"procfs_path" toml:"procfs_path" yaml:"procfs_path"`
	MetricsEnabled       []string `json:"metrics_enabled" toml:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsDisabled      []string `json:"metrics_disabled" toml:"metrics_disabled" yaml:"metrics_disabled"`
	MetricsDefaultStatus string   `json:"metrics_default_status" toml:"metrics_default_status" toml:"metrics_default_status"`
	RunTTL               string   `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewVMCollector creates new procfs cpu collector
func NewVMCollector(cfgBaseName string) (collector.Collector, error) {
	pkgID := "procfs.vm"
	procFile := "meminfo"
	c := VM{}
	c.id = "vm"
	c.procFSPath = "/proc"
	c.file = filepath.Join(c.procFSPath, procFile)
	c.logger = log.With().Str("pkg", pkgID).Logger()
	c.metricStatus = map[string]bool{}
	c.metricDefaultActive = true

	if cfgBaseName == "" {
		if _, err := os.Stat(c.file); err != nil {
			return nil, errors.Wrap(err, pkgID)
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
		return nil, errors.Wrap(err, pkgID+" config")
	}

	c.logger.Debug().Str("base", cfgBaseName).Interface("config", opts).Msg("loaded config")

	if opts.ID != "" {
		c.id = opts.ID
	}

	if opts.ProcFSPath != "" {
		c.procFSPath = opts.ProcFSPath
		c.file = filepath.Join(c.procFSPath, procFile)
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
			return nil, errors.Errorf(pkgID+" invalid metric default status (%s)", opts.MetricsDefaultStatus)
		}
	}

	if opts.RunTTL != "" {
		dur, err := time.ParseDuration(opts.RunTTL)
		if err != nil {
			return nil, errors.Wrap(err, pkgID+" parsing run_ttl")
		}
		c.runTTL = dur
	}

	if _, err := os.Stat(c.file); os.IsNotExist(err) {
		return nil, errors.Wrap(err, pkgID)
	}

	return &c, nil
}

// Collect metrics from the procfs resource
func (c *VM) Collect() error {
	pkgID := "procfs.vm"
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
		return errors.Wrap(err, pkgID)
	}

	if err := c.parseVMstats(&metrics); err != nil {
		c.setStatus(metrics, err)
		return errors.Wrap(err, pkgID)
	}

	c.setStatus(metrics, nil)
	return nil
}

func (c *VM) parseMemstats(metrics *cgm.Metrics) error {
	pkgID := "procfs.vm"
	f, err := os.Open(c.file)
	if err != nil {
		return errors.Wrap(err, pkgID)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	stats := make(map[string]uint64)
	for scanner.Scan() {

		line := strings.TrimSpace(scanner.Text())
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
			c.logger.Warn().Err(err).Msg(pkgID + " parsing field " + name)
			continue
		}

		if strings.ToLower(units) == "kb" {
			v *= uint64(1024)
		}

		stats[name] = v
	}

	if err := scanner.Err(); err != nil {
		return errors.Wrapf(err, pkgID+" parsing %s", f.Name())
	}

	var memTotal, memFree, memCached, memBuffers, swapTotal, swapFree uint64
	for metricName, mval := range stats {
		pfx := c.id + metricNameSeparator + "memory"
		mname := strings.ToLower(metricName)
		mtype := "L"
		switch metricName {
		case "MemTotal":
			mname = "total"
			memTotal = mval
		case "MemFree": // see memTotalFree below
			mname = "unused"
			memFree = mval
		case "MemAvailable":
			mname = "available"
		case "Buffers":
			memBuffers = mval
		case "Cached":
			memCached = mval
		case "SwapCached":
			pfx = c.id + metricNameSeparator + "swap"
			mname = "cached"
		case "Active(anon)":
			mname = "active_anon"
		case "Inactive(anon)":
			mname = "inactive_anon"
		case "Active(file)":
			mname = "active_file"
		case "Inactive(file)":
			mname = "inactive_file"
		case "SwapTotal":
			pfx = c.id + metricNameSeparator + "swap"
			mname = "total"
			swapTotal = mval
		case "SwapFree":
			pfx = c.id + metricNameSeparator + "swap"
			mname = "free"
			swapFree = mval
		case "AnonPages":
			mname = "anon_pages"
		case "SReclaimable":
			mname = "slab_reclaimable"
		case "SUnreclaim":
			mname = "slab_unreclaimable"
		case "KernelStack":
			mname = "kernel_stack"
		case "PageTables":
			mname = "page_tables"
		case "WritebackTmp":
			mname = "writeback_tmp"
		case "CommitLimit":
			mname = "commit_limit"
		case "VmallocTotal":
			mname = "vmalloc_total"
		case "VmallocUsed":
			mname = "vmalloc_used"
		case "VmallocChunk":
			mname = "vmalloc_chunk"
		case "HardwareCorrupted":
			mname = "hardware_corrupted"
		case "AnonHugePages":
			mname = "hugepages_anon"
		case "Hugepagesize":
			mname = "hugepage_size"
		case "DirectMap4k":
			mname = "direct_map_4K"
		case "DirectMap2M":
			mname = "direct_map_2M"
		}
		if mname != "" {
			c.addMetric(metrics, pfx, mname, mtype, mval)
		}
	}

	memFreeTotal := memFree + memBuffers + memCached
	memUsed := memTotal - memFreeTotal
	memFreePct := (float64(memFreeTotal) / float64(memTotal)) * 100
	memUsedPct := (float64(memUsed) / float64(memTotal)) * 100

	swapUsed := swapTotal - swapFree
	swapFreePct := 0.0
	swapUsedPct := 0.0
	if swapTotal > 0 {
		swapFreePct = (float64(swapFree) / float64(swapTotal)) * 100
		swapUsedPct = (float64(swapUsed) / float64(swapTotal)) * 100
	}

	pfx := c.id + metricNameSeparator + "memory"
	c.addMetric(metrics, pfx, "free", "L", memFreeTotal)
	c.addMetric(metrics, pfx, "used", "L", memUsed)
	c.addMetric(metrics, pfx, "free_percent", "n", memFreePct)
	c.addMetric(metrics, pfx, "used_percent", "n", memUsedPct)

	pfx = c.id + metricNameSeparator + "swap"
	c.addMetric(metrics, pfx, "used", "L", swapUsed)
	c.addMetric(metrics, pfx, "free_percent", "n", swapFreePct)
	c.addMetric(metrics, pfx, "used_percent", "n", swapUsedPct)

	return nil
}

func (c *VM) parseVMstats(metrics *cgm.Metrics) error {
	pkgID := "procfs.vm"
	file := strings.Replace(c.file, "meminfo", "vmstat", -1)
	f, err := os.Open(file)
	if err != nil {
		return errors.Wrap(err, pkgID)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	var pgFaults, pgMajorFaults, pgScan uint64
	pfx := c.id + metricNameSeparator + "info"
	for scanner.Scan() {

		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)

		if len(fields) != 2 {
			continue
		}

		switch {
		case fields[0] == "pgfault":
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				c.logger.Warn().Err(err).Msg(pkgID + " parsing field " + fields[0])
				continue
			}
			pgFaults = v
		case fields[0] == "pgmajfault":
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				c.logger.Warn().Err(err).Msg(pkgID + " parsing field " + fields[0])
				continue
			}
			pgMajorFaults = v
		case strings.HasPrefix(fields[0], "pswp"):
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				c.logger.Warn().Err(err).Msg(pkgID + " parsing field " + fields[0])
				continue
			}
			c.addMetric(metrics, pfx, fields[0], "L", v)
		case strings.HasPrefix(fields[0], "pgscan"):
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				c.logger.Warn().Err(err).Msg(pkgID + " parsing field " + fields[0])
				continue
			}
			pgScan += v
		default:
			// ignore
		}
	}

	if err := scanner.Err(); err != nil {
		return errors.Wrapf(err, pkgID+" parsing %s", f.Name())
	}

	c.addMetric(metrics, pfx, "page_fault", "L", pgFaults)
	c.addMetric(metrics, pfx, "page_fault"+metricNameSeparator+"minor", "L", pgFaults-pgMajorFaults)
	c.addMetric(metrics, pfx, "page_fault"+metricNameSeparator+"major", "L", pgMajorFaults)
	c.addMetric(metrics, pfx, "pg_scan", "L", pgScan)

	return nil
}
