// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build windows

package wmi

import (
	"regexp"
	"strconv"
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

// Win32_PerfRawData_Tcpip_TCPv4 defines the metrics to collect
type Win32_PerfRawData_Tcpip_TCPv4 struct {
	ConnectionFailures          uint32
	ConnectionsActive           uint32
	ConnectionsEstablished      uint32
	ConnectionsPassive          uint32
	ConnectionsReset            uint32
	SegmentsPersec              uint32
	SegmentsReceivedPersec      uint32
	SegmentsRetransmittedPersec uint32
	SegmentsSentPersec          uint32
}

// Win32_PerfRawData_Tcpip_TCPv6 defines the metrics to collect
type Win32_PerfRawData_Tcpip_TCPv6 struct {
	ConnectionFailures          uint32
	ConnectionsActive           uint32
	ConnectionsEstablished      uint32
	ConnectionsPassive          uint32
	ConnectionsReset            uint32
	SegmentsPersec              uint32
	SegmentsReceivedPersec      uint32
	SegmentsRetransmittedPersec uint32
	SegmentsSentPersec          uint32
}

// NetTCP metrics from the Windows Management Interface (wmi)
type NetTCP struct {
	wmicommon
	ipv4Enabled bool
	ipv6Enabled bool
}

// NetTCPOptions defines what elements can be overriden in a config file
type NetTCPOptions struct {
	ID                   string   `json:"id" toml:"id" yaml:"id"`
	MetricsEnabled       []string `json:"metrics_enabled" toml:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsDisabled      []string `json:"metrics_disabled" toml:"metrics_disabled" yaml:"metrics_disabled"`
	MetricsDefaultStatus string   `json:"metrics_default_status" toml:"metrics_default_status" toml:"metrics_default_status"`
	MetricNameRegex      string   `json:"metric_name_regex" toml:"metric_name_regex" yaml:"metric_name_regex"`
	MetricNameChar       string   `json:"metric_name_char" toml:"metric_name_char" yaml:"metric_name_char"`
	RunTTL               string   `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
	EnableIPv4           string   `json:"enable_ipv4" toml:"enable_ipv4" yaml:"enable_ipv4"`
	EnableIPv6           string   `json:"enable_ipv6" toml:"enable_ipv6" yaml:"enable_ipv6"`
}

// NewNetTCPCollector creates new wmi collector
func NewNetTCPCollector(cfgBaseName string) (collector.Collector, error) {
	c := NetTCP{}
	c.id = "net_tcp"
	c.logger = log.With().Str("pkg", "builtins.wmi."+c.id).Logger()
	c.metricDefaultActive = true
	c.metricNameChar = defaultMetricChar
	c.metricNameRegex = defaultMetricNameRegex
	c.metricStatus = map[string]bool{}

	c.ipv4Enabled = true
	c.ipv6Enabled = true

	if cfgBaseName == "" {
		return &c, nil
	}

	var cfg NetTCPOptions
	err := config.LoadConfigFile(cfgBaseName, &cfg)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Debug().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, errors.Wrap(err, "wmi.net_tcp config")
	}

	c.logger.Debug().Interface("config", cfg).Msg("loaded config")

	if cfg.EnableIPv4 != "" {
		ipv4, err := strconv.ParseBool(cfg.EnableIPv4)
		if err != nil {
			return nil, errors.Wrap(err, "wmi.processor parsing enable_ipv4")
		}
		c.ipv4Enabled = ipv4
	}

	if cfg.EnableIPv6 != "" {
		ipv6, err := strconv.ParseBool(cfg.EnableIPv6)
		if err != nil {
			return nil, errors.Wrap(err, "wmi.processor parsing enable_ipv6")
		}
		c.ipv6Enabled = ipv6
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
			return nil, errors.Errorf("wmi.net_tcp invalid metric default status (%s)", cfg.MetricsDefaultStatus)
		}
	}

	if cfg.MetricNameRegex != "" {
		rx, err := regexp.Compile(cfg.MetricNameRegex)
		if err != nil {
			return nil, errors.Wrapf(err, "wmi.net_tcp compile metric_name_regex")
		}
		c.metricNameRegex = rx
	}

	if cfg.MetricNameChar != "" {
		c.metricNameChar = cfg.MetricNameChar
	}

	if cfg.RunTTL != "" {
		dur, err := time.ParseDuration(cfg.RunTTL)
		if err != nil {
			return nil, errors.Wrap(err, "wmi.net_tcp parsing run_ttl")
		}
		c.runTTL = dur
	}

	return &c, nil
}

// Collect metrics from the wmi resource
func (c *NetTCP) Collect() error {
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

	if c.ipv4Enabled {
		var dst []Win32_PerfRawData_Tcpip_TCPv4
		qry := wmi.CreateQuery(dst, "")
		if err := wmi.Query(qry, &dst); err != nil {
			c.logger.Error().Err(err).Str("query", qry).Msg("wmi error")
			c.setStatus(metrics, err)
			return errors.Wrap(err, "wmi.net_tcp")
		}

		for _, item := range dst {
			pfx := c.id + metricNameSeparator + "v4"
			d := structs.Map(item) // there is only one NetTCP output

			for name, val := range d {
				if name == nameFieldName {
					continue
				}
				c.addMetric(&metrics, pfx, name, "L", val)
			}
		}
	}

	if c.ipv6Enabled {
		var dst []Win32_PerfRawData_Tcpip_TCPv6
		qry := wmi.CreateQuery(dst, "")
		if err := wmi.Query(qry, &dst); err != nil {
			c.logger.Error().Err(err).Str("query", qry).Msg("wmi error")
			c.setStatus(metrics, err)
			return errors.Wrap(err, "wmi.net_tcp")
		}

		for _, item := range dst {
			pfx := c.id + metricNameSeparator + "v6"
			d := structs.Map(item) // there is only one NetTCP output

			for name, val := range d {
				if name == nameFieldName {
					continue
				}
				c.addMetric(&metrics, pfx, name, "L", val)
			}
		}
	}

	c.setStatus(metrics, nil)
	return nil
}
