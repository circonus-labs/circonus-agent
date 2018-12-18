// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build linux

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
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// VM metrics from the Linux ProcFS
type VM struct {
	pfscommon
}

// vmOptions defines what elements can be overridden in a config file
type vmOptions struct {
	// common
	ID                   string   `json:"id" toml:"id" yaml:"id"`
	ProcFSPath           string   `json:"procfs_path" toml:"procfs_path" yaml:"procfs_path"`
	MetricsEnabled       []string `json:"metrics_enabled" toml:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsDisabled      []string `json:"metrics_disabled" toml:"metrics_disabled" yaml:"metrics_disabled"`
	MetricsDefaultStatus string   `json:"metrics_default_status" toml:"metrics_default_status" toml:"metrics_default_status"`
	RunTTL               string   `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewVMCollector creates new procfs vm collector
func NewVMCollector(cfgBaseName, procFSPath string) (collector.Collector, error) {
	procFile := "meminfo"

	c := VM{}
	c.id = VM_NAME
	c.pkgID = PFS_PREFIX + c.id
	c.procFSPath = procFSPath
	c.file = filepath.Join(c.procFSPath, procFile)
	c.logger = log.With().Str("pkg", c.pkgID).Logger()
	c.metricStatus = map[string]bool{}
	c.metricDefaultActive = true
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
	f, err := os.Open(c.file)
	if err != nil {
		return err
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
			c.logger.Warn().Err(err).Msg("parsing field " + name)
			continue
		}

		if strings.ToLower(units) == "kb" {
			v *= uint64(1024)
		}

		stats[name] = v
	}

	if err := scanner.Err(); err != nil {
		return errors.Wrapf(err, "parsing %s", f.Name())
	}

	var memTotal, memFree, memCached, memBuffers, memSReclaimable, memShared, swapTotal, swapFree uint64
	for metricName, mval := range stats {
		pfx := c.id + metricNameSeparator + "meminfo"
		mname := metricName
		mtype := "L"
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
		c.addMetric(metrics, pfx, mname, mtype, mval)
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

	memFreePct := (float64(memFreeTotal) / float64(memTotal))
	memUsedPct := (float64(memUsed) / float64(memTotal))

	swapUsed := swapTotal - swapFree
	swapFreePct := 0.0
	swapUsedPct := 0.0
	if swapTotal > 0 {
		swapFreePct = (float64(swapFree) / float64(swapTotal))
		swapUsedPct = (float64(swapUsed) / float64(swapTotal))
	}

	pfx := c.id + metricNameSeparator + "memory"
	c.addMetric(metrics, pfx, "free", "L", memFreeTotal)
	c.addMetric(metrics, pfx, "free_percent", "n", memFreePct*100)
	c.addMetric(metrics, pfx, "percent_free", "n", memFreePct)
	c.addMetric(metrics, pfx, "percent_used", "n", memUsedPct)
	c.addMetric(metrics, pfx, "total", "L", memTotal)
	c.addMetric(metrics, pfx, "used", "L", memUsed)
	c.addMetric(metrics, pfx, "used_percent", "n", memUsedPct*100)

	pfx = c.id + metricNameSeparator + "swap"
	c.addMetric(metrics, pfx, "free", "L", swapTotal-swapUsed)
	c.addMetric(metrics, pfx, "free_percent", "n", swapFreePct*100)
	c.addMetric(metrics, pfx, "percent_free", "n", swapFreePct)
	c.addMetric(metrics, pfx, "percent_used", "n", swapUsedPct)
	c.addMetric(metrics, pfx, "total", "L", swapTotal)
	c.addMetric(metrics, pfx, "used", "L", swapUsed)
	c.addMetric(metrics, pfx, "used_percent", "n", swapUsedPct*100)

	return nil
}

func (c *VM) parseVMstats(metrics *cgm.Metrics) error {
	file := strings.Replace(c.file, "meminfo", "vmstat", -1)
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	var pgFaults, pgMajorFaults, pgScan uint64
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
			c.addMetric(metrics, c.id+metricNameSeparator+"vmstat", fields[0], "L", v)

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

	if err := scanner.Err(); err != nil {
		return errors.Wrapf(err, "parsing %s", f.Name())
	}

	pfx := c.id + metricNameSeparator + "info"
	c.addMetric(metrics, pfx, "page_fault", "L", pgFaults)
	c.addMetric(metrics, pfx, "page_fault"+metricNameSeparator+"major", "L", pgMajorFaults)
	c.addMetric(metrics, pfx, "page_fault"+metricNameSeparator+"minor", "L", pgFaults-pgMajorFaults)
	c.addMetric(metrics, pfx, "page_scan", "L", pgScan)

	return nil
}
