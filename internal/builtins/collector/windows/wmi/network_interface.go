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

// Win32_PerfRawData_Tcpip_NetworkInterface defines the metrics to collect
type Win32_PerfRawData_Tcpip_NetworkInterface struct {
	BytesReceivedPersec             uint64
	BytesSentPersec                 uint64
	BytesTotalPersec                uint64
	CurrentBandwidth                uint64
	Name                            string
	OffloadedConnections            uint64
	OutputQueueLength               uint64
	PacketsOutboundDiscarded        uint64
	PacketsOutboundErrors           uint64
	PacketsPersec                   uint64
	PacketsReceivedDiscarded        uint64
	PacketsReceivedErrors           uint64
	PacketsReceivedNonUnicastPersec uint64
	PacketsReceivedPersec           uint64
	PacketsReceivedUnicastPersec    uint64
	PacketsReceivedUnknown          uint64
	PacketsSentNonUnicastPersec     uint64
	PacketsSentPersec               uint64
	PacketsSentUnicastPersec        uint64
	TCPActiveRSCConnections         uint64
	TCPRSCAveragePacketSize         uint64
	TCPRSCCoalescedPacketsPersec    uint64
	TCPRSCExceptionsPersec          uint64
}

// NetInterface metrics from the Windows Management Interface (wmi)
type NetInterface struct {
	wmicommon
	include *regexp.Regexp
	exclude *regexp.Regexp
}

// netInterfaceOptions defines what elements can be overriden in a config file
type netInterfaceOptions struct {
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

// NewNetInterfaceCollector creates new wmi collector
func NewNetInterfaceCollector(cfgBaseName string) (collector.Collector, error) {
	c := NetInterface{}
	c.id = "network_interface"
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

	var cfg netInterfaceOptions
	err := config.LoadConfigFile(cfgBaseName, &cfg)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Debug().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, errors.Wrap(err, "wmi.network_interface config")
	}

	c.logger.Debug().Interface("config", cfg).Msg("loaded config")

	// include regex
	if cfg.IncludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, cfg.IncludeRegex))
		if err != nil {
			return nil, errors.Wrap(err, "wmi.network_interface compiling include regex")
		}
		c.include = rx
	}

	// exclude regex
	if cfg.ExcludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, cfg.ExcludeRegex))
		if err != nil {
			return nil, errors.Wrap(err, "wmi.network_interface compiling exclude regex")
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
			return nil, errors.Errorf("wmi.network_interface invalid metric default status (%s)", cfg.MetricsDefaultStatus)
		}
	}

	if cfg.MetricNameRegex != "" {
		rx, err := regexp.Compile(cfg.MetricNameRegex)
		if err != nil {
			return nil, errors.Wrapf(err, "wmi.network_interface compile metric_name_regex")
		}
		c.metricNameRegex = rx
	}

	if cfg.MetricNameChar != "" {
		c.metricNameChar = cfg.MetricNameChar
	}

	if cfg.RunTTL != "" {
		dur, err := time.ParseDuration(cfg.RunTTL)
		if err != nil {
			return nil, errors.Wrap(err, "wmi.network_interface parsing run_ttl")
		}
		c.runTTL = dur
	}

	return &c, nil
}

// Collect metrics from the wmi resource
func (c *NetInterface) Collect() error {
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

	var dst []Win32_PerfRawData_Tcpip_NetworkInterface
	qry := wmi.CreateQuery(dst, "")
	if err := wmi.Query(qry, &dst); err != nil {
		c.logger.Error().Err(err).Str("query", qry).Msg("wmi error")
		c.setStatus(metrics, err)
		return errors.Wrap(err, "wmi.network_interface")
	}

	for _, item := range dst {

		// apply include/exclude to CLEAN item name
		itemName := c.cleanName(item.Name)
		if c.exclude.MatchString(itemName) || !c.include.MatchString(itemName) {
			continue
		}

		// adjust prefix, add item name
		pfx := c.id
		if strings.Contains(item.Name, "_Total") { // use the unclean name
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
