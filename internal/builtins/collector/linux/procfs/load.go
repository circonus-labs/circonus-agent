// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build linux

package procfs

import (
	"os"
	"path/filepath"
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

// Load metrics from the Linux ProcFS (actually from unix.Sysinfo call)
type Load struct {
	common
	processStatsFile string
}

// loadOptions defines what elements can be overridden in a config file
type loadOptions struct {
	// common
	ID                   string   `json:"id" toml:"id" yaml:"id"`
	ProcFSPath           string   `json:"procfs_path" toml:"procfs_path" yaml:"procfs_path"`
	MetricsEnabled       []string `json:"metrics_enabled" toml:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsDisabled      []string `json:"metrics_disabled" toml:"metrics_disabled" yaml:"metrics_disabled"`
	MetricsDefaultStatus string   `json:"metrics_default_status" toml:"metrics_default_status" toml:"metrics_default_status"`
	RunTTL               string   `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewLoadCollector creates new procfs load collector
func NewLoadCollector(cfgBaseName, procFSPath string) (collector.Collector, error) {
	procFile := NameLoad

	c := Load{}
	c.id = NameLoad
	c.pkgID = PKG_NAME + "." + c.id
	c.logger = log.With().Str("pkg", PKG_NAME).Str("id", c.id).Logger()
	c.procFSPath = procFSPath
	c.file = filepath.Join(c.procFSPath, procFile)
	c.processStatsFile = filepath.Join(c.procFSPath, "stat")
	c.baseTags = tags.FromList(tags.GetBaseTags())

	if cfgBaseName == "" {
		if _, err := os.Stat(c.file); os.IsNotExist(err) {
			return nil, errors.Wrap(err, c.pkgID)
		}
		return &c, nil
	}

	var opts loadOptions
	err := config.LoadConfigFile(cfgBaseName, &opts)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, errors.Wrapf(err, "%s config", c.pkgID)
	}

	c.logger.Debug().Interface("config", opts).Msg("loaded config")

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
func (c *Load) Collect() error {
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

	tagUnitsProcesses := tags.Tag{Category: "units", Value: "processes"}

	{
		// load metrics
		lines, err := c.readFile(c.file)
		if err != nil {
			c.setStatus(metrics, err)
			return errors.Wrap(err, c.pkgID)
		}

		metricType := "n"
		tagList := tags.Tags{tagUnitsProcesses}
		for _, l := range lines {
			line := string(l)
			fields := strings.Fields(line)

			if len(fields) < 3 {
				c.logger.Warn().Int("fields", len(fields)).Msg("invalid number of fields")
				continue
			}

			if v, err := strconv.ParseFloat(fields[0], 64); err != nil {
				c.logger.Warn().Err(err).Msg("parsing 1min field")
				continue
			} else {
				c.addMetric(&metrics, "", "load_1min", metricType, v, tagList)
			}

			if v, err := strconv.ParseFloat(fields[1], 64); err != nil {
				c.logger.Warn().Err(err).Msg("parsing 5min field")
				continue
			} else {
				c.addMetric(&metrics, "", "load_5min", metricType, v, tagList)
			}

			if v, err := strconv.ParseFloat(fields[2], 64); err != nil {
				c.logger.Warn().Err(err).Msg("parsing 15min field")
				continue
			} else {
				c.addMetric(&metrics, "", "load_15min", metricType, v, tagList)
			}
		}
	}

	{
		// process metrics
		lines, err := c.readFile(c.processStatsFile)
		if err != nil {
			c.setStatus(metrics, err)
			return errors.Wrap(err, c.pkgID)
		}

		var processes, running, blocked, ctxswitch uint64

		for _, l := range lines {
			var lineErr error
			line := string(l)
			fields := strings.Fields(line)

			switch fields[0] {
			case "processes":
				processes, lineErr = strconv.ParseUint(fields[1], 10, 64)

			case "procs_running":
				running, lineErr = strconv.ParseUint(fields[1], 10, 64)

			case "procs_blocked":
				blocked, lineErr = strconv.ParseUint(fields[1], 10, 64)

			case "ctxt":
				ctxswitch, lineErr = strconv.ParseUint(fields[1], 10, 64)
			default:
				continue
			}

			if lineErr != nil {
				c.setStatus(metrics, err)
				return errors.Wrapf(err, "%s parsing %s", c.pkgID, fields[0])
			}
		}

		metricType := "L"

		{
			tagList := tags.Tags{tagUnitsProcesses}
			c.addMetric(&metrics, "", "total", metricType, processes, tagList)
			c.addMetric(&metrics, "", "running", metricType, running, tagList)
			c.addMetric(&metrics, "", "blocked", metricType, blocked, tagList)
		}

		{
			tagList := tags.Tags{tags.Tag{Category: "units", Value: "switches"}}
			c.addMetric(&metrics, "", "ctxt", metricType, ctxswitch, tagList)
		}

	}

	c.setStatus(metrics, nil)
	return nil
}
