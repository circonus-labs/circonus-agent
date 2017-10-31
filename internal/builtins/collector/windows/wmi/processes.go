// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build windows

package wmi

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/StackExchange/wmi"
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	cgm "github.com/circonus-labs/circonus-gometrics"
	"github.com/fatih/structs"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// Win32_PerfFormattedData_PerfProc_Process defines the metrics to collect
type Win32_PerfFormattedData_PerfProc_Process struct {
	CreatingProcessID       uint32
	ElapsedTime             uint64
	HandleCount             uint32
	IDProcess               uint32
	IODataBytesPersec       uint64
	IODataOperationsPersec  uint64
	IOOtherBytesPersec      uint64
	IOOtherOperationsPersec uint64
	IOReadBytesPersec       uint64
	IOReadOperationsPersec  uint64
	IOWriteBytesPersec      uint64
	IOWriteOperationsPersec uint64
	Name                    string
	PageFaultsPersec        uint32
	PageFileBytes           uint64
	PageFileBytesPeak       uint64
	PercentPrivilegedTime   uint64
	PercentProcessorTime    uint64
	PercentUserTime         uint64
	PoolNonpagedBytes       uint32
	PoolPagedBytes          uint32
	PriorityBase            uint32
	PrivateBytes            uint64
	ThreadCount             uint32
	VirtualBytes            uint64
	VirtualBytesPeak        uint64
	WorkingSet              uint64
	WorkingSetPeak          uint64
	WorkingSetPrivate       uint64
}

// Processes metrics from the Windows Management Interface (wmi)
type Processes struct {
	wmicommon
	include *regexp.Regexp
	exclude *regexp.Regexp
}

// ProcessesOptions defines what elements can be overriden in a config file
type ProcessesOptions struct {
	ID                   string   `json:"id" toml:"id" yaml:"id"`
	IncludeRegex         string   `json:"include_regex" toml:"include_regex" yaml:"include_regex"`
	ExcludeRegex         string   `json:"exclude_regex" toml:"exclude_regex" yaml:"exclude_regex"`
	MetricsEnabled       []string `json:"metrics_enabled" toml:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsDisabled      []string `json:"metrics_disabled" toml:"metrics_disabled" yaml:"metrics_disabled"`
	MetricsDefaultStatus string   `json:"metrics_default_status" toml:"metrics_default_status" toml:"metrics_default_status"`
	MetricNameRegex      string   `json:"metric_name_regex" toml:"metric_name_regex" yaml:"metric_name_regex"`
	MetricNameChar       string   `json:"metric_name_char" toml:"metric_name_char" yaml:"metric_name_char"`
	RunTTL               string   `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewProcessesCollector creates new wmi collector
func NewProcessesCollector(cfgBaseName string) (collector.Collector, error) {
	c := Processes{}
	c.id = "processes"
	c.logger = log.With().Str("pkg", "builtins.wmi."+c.id).Logger()
	c.metricDefaultActive = true
	c.metricNameChar = defaultMetricChar
	c.metricNameRegex = defaultMetricNameRegex
	c.metricStatus = map[string]bool{}

	c.include = defaultIncludeRegex
	c.exclude = defaultExcludeRegex

	if cfgBaseName == "" {
		return &c, nil
	}

	var cfg ProcessesOptions
	err := config.LoadConfigFile(cfgBaseName, &cfg)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Debug().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, errors.Wrap(err, "wmi.processes config")
	}

	c.logger.Debug().Interface("config", cfg).Msg("loaded config")

	// include regex
	if cfg.IncludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, cfg.IncludeRegex))
		if err != nil {
			return nil, errors.Wrap(err, "wmi.physical_disk compiling include regex")
		}
		c.include = rx
	}

	// exclude regex
	if cfg.ExcludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, cfg.ExcludeRegex))
		if err != nil {
			return nil, errors.Wrap(err, "wmi.physical_disk compiling exclude regex")
		}
		c.exclude = rx
	}

	if cfg.ID != "" {
		c.id = cfg.ID
	}

	if len(cfg.MetricsEnabled) > 0 {
		for _, name := range cfg.MetricsEnabled {
			c.metricStatus[name] = true
		}
	}
	if len(cfg.MetricsDisabled) > 0 {
		for _, name := range cfg.MetricsDisabled {
			c.metricStatus[name] = false
		}
	}

	if cfg.MetricsDefaultStatus != "" {
		if ok, _ := regexp.MatchString(`^(enabled|disabled)$`, strings.ToLower(cfg.MetricsDefaultStatus)); ok {
			c.metricDefaultActive = strings.ToLower(cfg.MetricsDefaultStatus) == metricStatusEnabled
		} else {
			return nil, errors.Errorf("wmi.processes invalid metric default status (%s)", cfg.MetricsDefaultStatus)
		}
	}

	if cfg.MetricNameRegex != "" {
		rx, err := regexp.Compile(cfg.MetricNameRegex)
		if err != nil {
			return nil, errors.Wrapf(err, "wmi.processes compile metric_name_regex")
		}
		c.metricNameRegex = rx
	}

	if cfg.MetricNameChar != "" {
		c.metricNameChar = cfg.MetricNameChar
	}

	if cfg.RunTTL != "" {
		dur, err := time.ParseDuration(cfg.RunTTL)
		if err != nil {
			return nil, errors.Wrap(err, "wmi.processes parsing run_ttl")
		}
		c.runTTL = dur
	}

	return &c, nil
}

// Collect metrics from the wmi resource
func (c *Processes) Collect() error {
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

	var dst []Win32_PerfFormattedData_PerfProc_Process
	qry := wmi.CreateQuery(dst, "")
	if err := wmi.Query(qry, &dst); err != nil {
		c.logger.Error().Err(err).Str("query", qry).Msg("wmi error")
		c.setStatus(metrics, err)
		return errors.Wrap(err, "wmi.processes")
	}

	for _, item := range dst {

		// apply include/exclude to CLEAN item name
		itemName := c.cleanName(item.Name)
		if c.exclude.MatchString(itemName) || !c.include.MatchString(itemName) {
			continue
		}

		// adjust prefix, add item name
		pfx := c.id
		if strings.Contains(item.Name, totalName) { // use the unclean name
			pfx += totalPrefix
		} else {
			pfx += metricNameSeparator + itemName
		}

		d := structs.Map(item)
		for name, val := range d {
			if name == nameFieldName {
				continue
			}
			c.addMetric(&metrics, pfx, name, "L", val)
		}
	}

	c.setStatus(metrics, nil)
	return nil
}
