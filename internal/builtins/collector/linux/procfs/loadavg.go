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
	cgm "github.com/circonus-labs/circonus-gometrics"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// Loadavg metrics from the Linux ProcFS (actually from unix.Sysinfo call)
type Loadavg struct {
	pfscommon
	file string
}

// loadavgOptions defines what elements can be overriden in a config file
type loadavgOptions struct {
	// common
	ID                   string   `json:"id" toml:"id" yaml:"id"`
	ProcFSPath           string   `json:"procfs_path" toml:"procfs_path" yaml:"procfs_path"`
	MetricsEnabled       []string `json:"metrics_enabled" toml:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsDisabled      []string `json:"metrics_disabled" toml:"metrics_disabled" yaml:"metrics_disabled"`
	MetricsDefaultStatus string   `json:"metrics_default_status" toml:"metrics_default_status" toml:"metrics_default_status"`
	RunTTL               string   `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewLoadavgCollector creates new procfs cpu collector
func NewLoadavgCollector(cfgBaseName string) (collector.Collector, error) {
	c := Loadavg{}
	c.id = "loadavg"
	c.procFSPath = "/proc"
	c.file = filepath.Join(c.procFSPath, "loadavg")
	c.logger = log.With().Str("pkg", "procfs.loadavg").Logger()
	c.metricStatus = map[string]bool{}
	c.metricDefaultActive = true

	if cfgBaseName == "" {
		if _, err := os.Stat(c.file); os.IsNotExist(err) {
			return nil, errors.Wrap(err, "procfs.loadavg")
		}
		return &c, nil
	}

	var opts loadavgOptions
	err := config.LoadConfigFile(cfgBaseName, &opts)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, errors.Wrap(err, "procfs.loadavg config")
	}

	c.logger.Debug().Interface("config", opts).Msg("loaded config")

	if opts.ID != "" {
		c.id = opts.ID
	}

	if opts.ProcFSPath != "" {
		c.procFSPath = opts.ProcFSPath
		c.file = filepath.Join(c.procFSPath, "loadavg")
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
			return nil, errors.Errorf("procfs.loadavg invalid metric default status (%s)", opts.MetricsDefaultStatus)
		}
	}

	if opts.RunTTL != "" {
		dur, err := time.ParseDuration(opts.RunTTL)
		if err != nil {
			return nil, errors.Wrap(err, "procfs.loadavg parsing run_ttl")
		}
		c.runTTL = dur
	}

	if _, err := os.Stat(c.file); os.IsNotExist(err) {
		return nil, errors.Wrap(err, "procfs.loadavg")
	}

	return &c, nil
}

// Collect metrics from the procfs resource
func (c *Loadavg) Collect() error {
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

	metricType := "n"
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		if len(fields) < 3 {
			c.logger.Warn().Int("fields", len(fields)).Msg("procfs.loadavg invalid number of fields")
			continue
		}

		if v, err := strconv.ParseFloat(fields[0], 64); err != nil {
			c.logger.Warn().Err(err).Msg("procfs.loadavg parsing 1min field")
			continue
		} else {
			c.addMetric(&metrics, c.id, "1", metricType, v)
		}

		if v, err := strconv.ParseFloat(fields[1], 64); err != nil {
			c.logger.Warn().Err(err).Msg("procfs.loadavg parsing 5min field")
			continue
		} else {
			c.addMetric(&metrics, c.id, "5", metricType, v)
		}

		if v, err := strconv.ParseFloat(fields[2], 64); err != nil {
			c.logger.Warn().Err(err).Msg("procfs.loadavg parsing 15min field")
			continue
		} else {
			c.addMetric(&metrics, c.id, "15", metricType, v)
		}
	}

	if err := scanner.Err(); err != nil {
		c.setStatus(metrics, err)
		return errors.Wrapf(err, "parsing %s", f.Name())
	}

	c.setStatus(metrics, nil)
	return nil
}
