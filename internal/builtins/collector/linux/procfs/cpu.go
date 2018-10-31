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
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// CPU metrics from the Linux ProcFS
type CPU struct {
	pfscommon
	numCPU        float64 // number of cpus
	clockNorm     float64 // cpu clock normalized to 100Hz tick rate
	reportAllCPUs bool    // OPT report all cpus (vs just total) may be overridden in config file
	file          string
}

// cpuOptions defines what elements can be overridden in a config file
type cpuOptions struct {
	// common
	ID                   string   `json:"id" toml:"id" yaml:"id"`
	ProcFSPath           string   `json:"procfs_path" toml:"procfs_path" yaml:"procfs_path"`
	MetricsEnabled       []string `json:"metrics_enabled" toml:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsDisabled      []string `json:"metrics_disabled" toml:"metrics_disabled" yaml:"metrics_disabled"`
	MetricsDefaultStatus string   `json:"metrics_default_status" toml:"metrics_default_status" toml:"metrics_default_status"`
	RunTTL               string   `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`

	// collector specific
	ClockHZ string `json:"clock_hz" toml:"clock_hz" yaml:"clock_hz"`
	AllCPU  string `json:"report_all_cpus" toml:"report_all_cpus" yaml:"report_all_cpus"`
}

// NewCPUCollector creates new procfs cpu collector
func NewCPUCollector(cfgBaseName, procFSPath string) (collector.Collector, error) {
	procFile := "stat"

	c := CPU{}
	c.id = CPU_NAME
	c.pkgID = PFS_PREFIX + c.id
	c.procFSPath = procFSPath
	c.file = filepath.Join(c.procFSPath, procFile)
	c.logger = log.With().Str("pkg", c.pkgID).Logger()
	c.metricStatus = map[string]bool{}
	c.metricDefaultActive = true

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
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, errors.Wrapf(err, "%s config", c.pkgID)
	}

	c.logger.Debug().Interface("config", opts).Msg("loaded config")

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
func (c *CPU) Collect() error {
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

	f, err := os.Open(c.file)
	if err != nil {
		c.setStatus(metrics, err)
		return errors.Wrap(err, c.pkgID)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {

		line := scanner.Text()
		fields := strings.Fields(line)

		switch {
		case fields[0] == "processes":
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				c.setStatus(metrics, err)
				return errors.Wrapf(err, "%s parsing %s", c.pkgID, fields[0])
			}
			c.addMetric(&metrics, c.id, fields[0], "L", v)

		case fields[0] == "procs_running":
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				c.setStatus(metrics, err)
				return errors.Wrapf(err, "%s parsing %s", c.pkgID, fields[0])
			}
			c.addMetric(&metrics, c.id, "procs_runnable", "L", v)

		case fields[0] == "procs_blocked":
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				c.setStatus(metrics, err)
				return errors.Wrapf(err, "%s parsing %s", c.pkgID, fields[0])
			}
			c.addMetric(&metrics, c.id, fields[0], "L", v)

		case fields[0] == "ctxt":
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				c.setStatus(metrics, err)
				return errors.Wrapf(err, "%s parsing %s", c.pkgID, fields[0])
			}
			c.addMetric(&metrics, c.id, "context_switch", "L", v)

		case strings.HasPrefix(fields[0], CPU_NAME):
			if fields[0] != CPU_NAME && !c.reportAllCPUs {
				continue
			}
			cpuMetrics, err := c.parseCPU(fields)
			if err != nil {
				c.setStatus(metrics, err)
				return errors.Wrapf(err, "%s parsing %s", c.pkgID, fields[0])
			}
			for mn, mv := range *cpuMetrics {
				c.addMetric(&metrics, c.id, mn, mv.Type, mv.Value)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		c.setStatus(metrics, err)
		return errors.Wrapf(err, "%s parsing %s", c.pkgID, f.Name())
	}

	c.setStatus(metrics, nil)
	return nil
}

func (c *CPU) parseCPU(fields []string) (*cgm.Metrics, error) {
	var numCPU float64
	var metricBase string

	if fields[0] == "cpu" {
		metricBase = ""
		numCPU = c.numCPU // aggregate cpu metrics
	} else {
		metricBase = fields[0] + metricNameSeparator
		numCPU = 1 // individual cpu metrics
	}

	metricType := "n" // resmon double

	userNormal, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return nil, err
	}

	userNice, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return nil, err
	}

	sys, err := strconv.ParseFloat(fields[3], 64)
	if err != nil {
		return nil, err
	}

	idleNormal, err := strconv.ParseFloat(fields[4], 64)
	if err != nil {
		return nil, err
	}

	waitIO, err := strconv.ParseFloat(fields[5], 64)
	if err != nil {
		return nil, err
	}

	irq, err := strconv.ParseFloat(fields[6], 64)
	if err != nil {
		return nil, err
	}

	softIRQ, err := strconv.ParseFloat(fields[7], 64)
	if err != nil {
		return nil, err
	}

	steal := float64(0)
	if len(fields) > 8 {
		v, err := strconv.ParseFloat(fields[8], 64)
		if err != nil {
			return nil, err
		}
		steal = v
	}

	guest := float64(0)
	if len(fields) > 9 {
		v, err := strconv.ParseFloat(fields[9], 64)
		if err != nil {
			return nil, err
		}
		guest = v
	}

	guestNice := float64(0)
	if len(fields) > 10 {
		v, err := strconv.ParseFloat(fields[10], 64)
		if err != nil {
			return nil, err
		}
		guestNice = v
	}

	metrics := cgm.Metrics{
		metricBase + "user": cgm.Metric{Type: metricType, Value: ((userNormal + userNice) / numCPU) / c.clockNorm},
		metricBase + "user" + metricNameSeparator + "normal":       cgm.Metric{Type: metricType, Value: (userNormal / numCPU) / c.clockNorm},
		metricBase + "user" + metricNameSeparator + "nice":         cgm.Metric{Type: metricType, Value: (userNice / numCPU) / c.clockNorm},
		metricBase + "kernel":                                      cgm.Metric{Type: metricType, Value: ((sys + guest + guestNice) / numCPU) / c.clockNorm},
		metricBase + "kernel" + metricNameSeparator + "sys":        cgm.Metric{Type: metricType, Value: (sys / numCPU) / c.clockNorm},
		metricBase + "kernel" + metricNameSeparator + "guest":      cgm.Metric{Type: metricType, Value: (guest / numCPU) / c.clockNorm},
		metricBase + "kernel" + metricNameSeparator + "guest_nice": cgm.Metric{Type: metricType, Value: (guestNice / numCPU) / c.clockNorm},
		metricBase + "idle":                                        cgm.Metric{Type: metricType, Value: ((idleNormal + steal) / numCPU) / c.clockNorm},
		metricBase + "idle" + metricNameSeparator + "normal":       cgm.Metric{Type: metricType, Value: (idleNormal / numCPU) / c.clockNorm},
		metricBase + "idle" + metricNameSeparator + "steal":        cgm.Metric{Type: metricType, Value: (steal / numCPU) / c.clockNorm},
		metricBase + "wait_io":                                     cgm.Metric{Type: metricType, Value: (waitIO / numCPU) / c.clockNorm},
		metricBase + "intr":                                        cgm.Metric{Type: metricType, Value: ((irq + softIRQ) / numCPU) / c.clockNorm},
		metricBase + "intr" + metricNameSeparator + "soft":         cgm.Metric{Type: metricType, Value: (irq / numCPU) / c.clockNorm},
		metricBase + "intr" + metricNameSeparator + "hard":         cgm.Metric{Type: metricType, Value: (softIRQ / numCPU) / c.clockNorm},
	}

	return &metrics, nil
}
