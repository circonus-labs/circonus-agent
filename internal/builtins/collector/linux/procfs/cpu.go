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
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	cgm "github.com/circonus-labs/circonus-gometrics"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// CPU metrics from the Linux ProcFS
type CPU struct {
	pfscommon
	numCPU        float64 // number of cpus
	clockHZ       float64 // OPT getconf CLK_TCK, may be overriden in config file
	reportAllCPUs bool    // OPT report all cpus (vs just total) may be overriden in config file
	file          string
}

// cpuOptions defines what elements can be overriden in a config file
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
func NewCPUCollector(cfgBaseName string) (collector.Collector, error) {
	c := CPU{}
	c.id = "cpu"
	c.procFSPath = "/proc"
	c.file = filepath.Join(c.procFSPath, "stat")
	c.logger = log.With().Str("pkg", "procfs.cpu").Logger()
	c.metricStatus = map[string]bool{}
	c.metricDefaultActive = true

	c.numCPU = float64(runtime.NumCPU())
	c.clockHZ = 100
	c.reportAllCPUs = true

	if cfgBaseName == "" {
		if _, err := os.Stat(c.file); os.IsNotExist(err) {
			return nil, errors.Wrap(err, "procfs.cpu")
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
		return nil, errors.Wrap(err, "procfs.cpu config")
	}

	c.logger.Debug().Interface("config", opts).Msg("loaded config")

	if opts.ClockHZ != "" {
		v, err := strconv.ParseFloat(opts.ClockHZ, 64)
		if err != nil {
			return nil, errors.Wrap(err, "procfs.cpu parsing clock_hz")
		}
		c.clockHZ = v
	}

	if opts.AllCPU != "" {
		rpt, err := strconv.ParseBool(opts.AllCPU)
		if err != nil {
			return nil, errors.Wrap(err, "procfs.cpu parsing report_all_cpus")
		}
		c.reportAllCPUs = rpt
	}

	if opts.ID != "" {
		c.id = opts.ID
	}

	if opts.ProcFSPath != "" {
		c.procFSPath = opts.ProcFSPath
		c.file = filepath.Join(c.procFSPath, "stat")
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
			return nil, errors.Errorf("procfs.cpu invalid metric default status (%s)", opts.MetricsDefaultStatus)
		}
	}

	if opts.RunTTL != "" {
		dur, err := time.ParseDuration(opts.RunTTL)
		if err != nil {
			return nil, errors.Wrap(err, "procfs.cpu parsing run_ttl")
		}
		c.runTTL = dur
	}

	if _, err := os.Stat(c.file); os.IsNotExist(err) {
		return nil, errors.Wrap(err, "procfs.cpu")
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
		return errors.Wrap(err, "procfs.cpu")
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
				return errors.Wrapf(err, "parsing %s", fields[0])
			}
			c.addMetric(&metrics, c.id, fields[0], "L", v)

		case fields[0] == "procs_running":
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				c.setStatus(metrics, err)
				return errors.Wrapf(err, "parsing %s", fields[0])
			}
			c.addMetric(&metrics, c.id, "procs_runnable", "L", v)

		case fields[0] == "procs_blocked":
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				c.setStatus(metrics, err)
				return errors.Wrapf(err, "parsing %s", fields[0])
			}
			c.addMetric(&metrics, c.id, fields[0], "L", v)

		case fields[0] == "ctxt":
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				c.setStatus(metrics, err)
				return errors.Wrapf(err, "parsing %s", fields[0])
			}
			c.addMetric(&metrics, c.id, "context_switch", "L", v)

		case strings.HasPrefix(fields[0], "cpu"):
			if fields[0] != "cpu" && !c.reportAllCPUs {
				continue
			}
			cpuMetrics, err := c.parseCPU(fields)
			if err != nil {
				c.setStatus(metrics, err)
				return errors.Wrapf(err, "parsing %s", fields[0])
			}
			for mn, mv := range *cpuMetrics {
				c.addMetric(&metrics, c.id, mn, mv.Type, mv.Value)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		c.setStatus(metrics, err)
		return errors.Wrapf(err, "parsing %s", f.Name())
	}

	c.setStatus(metrics, nil)
	return nil
}

func (c *CPU) parseCPU(fields []string) (*cgm.Metrics, error) {
	var numCPU float64
	var metricBase string

	if fields[0] == "cpu" {
		metricBase = "total"
		numCPU = c.numCPU // aggregate cpu metrics
	} else {
		metricBase = fields[0]
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
		metricBase + "`user":              cgm.Metric{Type: metricType, Value: ((userNormal + userNice) / numCPU) / c.clockHZ},
		metricBase + "`user`normal":       cgm.Metric{Type: metricType, Value: (userNormal / numCPU) / c.clockHZ},
		metricBase + "`user`nice":         cgm.Metric{Type: metricType, Value: (userNice / numCPU) / c.clockHZ},
		metricBase + "`kernel":            cgm.Metric{Type: metricType, Value: ((sys + guest + guestNice) / numCPU) / c.clockHZ},
		metricBase + "`kernel`sys":        cgm.Metric{Type: metricType, Value: (sys / numCPU) / c.clockHZ},
		metricBase + "`kernel`guest":      cgm.Metric{Type: metricType, Value: (guest / numCPU) / c.clockHZ},
		metricBase + "`kernel`guest_nice": cgm.Metric{Type: metricType, Value: (guestNice / numCPU) / c.clockHZ},
		metricBase + "`idle":              cgm.Metric{Type: metricType, Value: ((idleNormal + steal) / numCPU) / c.clockHZ},
		metricBase + "`idle`normal":       cgm.Metric{Type: metricType, Value: (idleNormal / numCPU) / c.clockHZ},
		metricBase + "`idle`steal":        cgm.Metric{Type: metricType, Value: (steal / numCPU) / c.clockHZ},
		metricBase + "`wait_io":           cgm.Metric{Type: metricType, Value: (waitIO / numCPU) / c.clockHZ},
		metricBase + "`intr":              cgm.Metric{Type: metricType, Value: ((irq + softIRQ) / numCPU) / c.clockHZ},
		metricBase + "`intr`soft":         cgm.Metric{Type: metricType, Value: (irq / numCPU) / c.clockHZ},
		metricBase + "`intr`hard":         cgm.Metric{Type: metricType, Value: (softIRQ / numCPU) / c.clockHZ},
	}

	return &metrics, nil
}
