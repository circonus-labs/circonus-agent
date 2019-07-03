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
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// Win32_PerfRawData_Tcpip_UDPv4 defines the metrics to collect
type Win32_PerfRawData_Tcpip_UDPv4 struct { //nolint: golint
	DatagramsNoPortPersec   uint32
	DatagramsPersec         uint32
	DatagramsReceivedErrors uint32
	DatagramsReceivedPersec uint32
	DatagramsSentPersec     uint32
}

// Win32_PerfRawData_Tcpip_UDPv6 defines the metrics to collect
type Win32_PerfRawData_Tcpip_UDPv6 struct { //nolint: golint
	DatagramsNoPortPersec   uint32
	DatagramsPersec         uint32
	DatagramsReceivedErrors uint32
	DatagramsReceivedPersec uint32
	DatagramsSentPersec     uint32
}

// NetUDP metrics from the Windows Management Interface (wmi)
type NetUDP struct {
	wmicommon
	ipv4Enabled bool
	ipv6Enabled bool
}

// NetUDPOptions defines what elements can be overridden in a config file
type NetUDPOptions struct {
	ID              string `json:"id" toml:"id" yaml:"id"`
	MetricNameRegex string `json:"metric_name_regex" toml:"metric_name_regex" yaml:"metric_name_regex"`
	MetricNameChar  string `json:"metric_name_char" toml:"metric_name_char" yaml:"metric_name_char"`
	RunTTL          string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
	EnableIPv4      string `json:"enable_ipv4" toml:"enable_ipv4" yaml:"enable_ipv4"`
	EnableIPv6      string `json:"enable_ipv6" toml:"enable_ipv6" yaml:"enable_ipv6"`
}

// NewNetUDPCollector creates new wmi collector
func NewNetUDPCollector(cfgBaseName string) (collector.Collector, error) {
	c := NetUDP{}
	c.id = "net_udp"
	c.pkgID = pkgName + "." + c.id
	c.logger = log.With().Str("pkg", pkgName).Str("id", c.id).Logger()
	c.metricNameChar = defaultMetricChar
	c.metricNameRegex = defaultMetricNameRegex
	c.baseTags = tags.FromList(tags.GetBaseTags())

	c.ipv4Enabled = true
	c.ipv6Enabled = true

	if cfgBaseName == "" {
		return &c, nil
	}

	var cfg NetUDPOptions
	err := config.LoadConfigFile(cfgBaseName, &cfg)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Debug().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, errors.Wrapf(err, "%s config", c.pkgID)
	}

	c.logger.Debug().Interface("config", cfg).Msg("loaded config")

	if cfg.EnableIPv4 != "" {
		ipv4, err := strconv.ParseBool(cfg.EnableIPv4)
		if err != nil {
			return nil, errors.Wrapf(err, "%s parsing enable_ipv4", c.pkgID)
		}
		c.ipv4Enabled = ipv4
	}

	if cfg.EnableIPv6 != "" {
		ipv6, err := strconv.ParseBool(cfg.EnableIPv6)
		if err != nil {
			return nil, errors.Wrapf(err, "%s parsing enable_ipv6", c.pkgID)
		}
		c.ipv6Enabled = ipv6
	}

	if cfg.ID != "" {
		c.id = cfg.ID
	}

	if cfg.MetricNameRegex != "" {
		rx, err := regexp.Compile(cfg.MetricNameRegex)
		if err != nil {
			return nil, errors.Wrapf(err, "%s compile metric_name_regex", c.pkgID)
		}
		c.metricNameRegex = rx
	}

	if cfg.MetricNameChar != "" {
		c.metricNameChar = cfg.MetricNameChar
	}

	if cfg.RunTTL != "" {
		dur, err := time.ParseDuration(cfg.RunTTL)
		if err != nil {
			return nil, errors.Wrapf(err, "%s parsing run_ttl", c.pkgID)
		}
		c.runTTL = dur
	}

	return &c, nil
}

// Collect metrics from the wmi resource
func (c *NetUDP) Collect() error {
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

	metricType := "I"
	tagUnitsDatagrams := cgm.Tag{Category: "units", Value: "datagrams"}

	if c.ipv4Enabled {
		var dst []Win32_PerfRawData_Tcpip_UDPv4
		qry := wmi.CreateQuery(dst, "")
		if err := wmi.Query(qry, &dst); err != nil {
			c.logger.Error().Err(err).Str("query", qry).Msg("wmi query error")
			c.setStatus(metrics, err)
			return errors.Wrap(err, c.pkgID)
		}

		if len(dst) > 1 {
			c.logger.Warn().Int("len", len(dst)).Msg("prot udp4 metrics has more than one SET of enteries")
		}

		protoTag := cgm.Tag{Category: "network-proto", Value: "udp4"}

		for _, item := range dst {
			_ = c.addMetric(&metrics, "", "DatagramsNoPortPersec", metricType, item.DatagramsNoPortPersec, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsPersec", metricType, item.DatagramsPersec, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsReceivedErrors", metricType, item.DatagramsReceivedErrors, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsReceivedPersec", metricType, item.DatagramsReceivedPersec, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsSentPersec", metricType, item.DatagramsSentPersec, cgm.Tags{protoTag, tagUnitsDatagrams})
		}
	}

	if c.ipv6Enabled {
		var dst []Win32_PerfRawData_Tcpip_UDPv6
		qry := wmi.CreateQuery(dst, "")
		if err := wmi.Query(qry, &dst); err != nil {
			c.logger.Error().Err(err).Str("query", qry).Msg("wmi query error")
			c.setStatus(metrics, err)
			return errors.Wrap(err, c.pkgID)
		}

		if len(dst) > 1 {
			c.logger.Warn().Int("len", len(dst)).Msg("prot udp6 metrics has more than one SET of enteries")
		}

		protoTag := cgm.Tag{Category: "network-proto", Value: "udp6"}

		for _, item := range dst {
			_ = c.addMetric(&metrics, "", "DatagramsNoPortPersec", metricType, item.DatagramsNoPortPersec, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsPersec", metricType, item.DatagramsPersec, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsReceivedErrors", metricType, item.DatagramsReceivedErrors, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsReceivedPersec", metricType, item.DatagramsReceivedPersec, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsSentPersec", metricType, item.DatagramsSentPersec, cgm.Tags{protoTag, tagUnitsDatagrams})
		}
	}

	c.setStatus(metrics, nil)
	return nil
}
