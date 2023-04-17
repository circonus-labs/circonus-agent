// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

//go:build windows
// +build windows

package wmi

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	// "github.com/StackExchange/wmi".
	"github.com/bi-zone/wmi"
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/rs/zerolog/log"
)

// Win32_PerfRawData_Tcpip_TCPv4 defines the metrics to collect.
type Win32_PerfRawData_Tcpip_TCPv4 struct { //nolint: revive
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

// Win32_PerfRawData_Tcpip_TCPv6 defines the metrics to collect.
type Win32_PerfRawData_Tcpip_TCPv6 struct { //nolint: revive
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

// NetTCP metrics from the Windows Management Interface (wmi).
type NetTCP struct {
	wmicommon
	ipv4Enabled bool
	ipv6Enabled bool
}

// NetTCPOptions defines what elements can be overridden in a config file.
type NetTCPOptions struct {
	ID              string `json:"id" toml:"id" yaml:"id"`
	MetricNameRegex string `json:"metric_name_regex" toml:"metric_name_regex" yaml:"metric_name_regex"`
	MetricNameChar  string `json:"metric_name_char" toml:"metric_name_char" yaml:"metric_name_char"`
	RunTTL          string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
	EnableIPv4      string `json:"enable_ipv4" toml:"enable_ipv4" yaml:"enable_ipv4"`
	EnableIPv6      string `json:"enable_ipv6" toml:"enable_ipv6" yaml:"enable_ipv6"`
}

// NewNetTCPCollector creates new wmi collector.
func NewNetTCPCollector(cfgBaseName string) (collector.Collector, error) {
	c := NetTCP{}
	c.id = "net_tcp"
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

	var cfg NetTCPOptions
	err := config.LoadConfigFile(cfgBaseName, &cfg)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Debug().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, fmt.Errorf("%s config: %w", c.pkgID, err)
	}

	c.logger.Debug().Interface("config", cfg).Msg("loaded config")

	if cfg.EnableIPv4 != "" {
		ipv4, err := strconv.ParseBool(cfg.EnableIPv4)
		if err != nil {
			return nil, fmt.Errorf("%s parsing enable_ipv4: %w", c.pkgID, err)
		}
		c.ipv4Enabled = ipv4
	}

	if cfg.EnableIPv6 != "" {
		ipv6, err := strconv.ParseBool(cfg.EnableIPv6)
		if err != nil {
			return nil, fmt.Errorf("%s parsing enable_ipv6: %w", c.pkgID, err)
		}
		c.ipv6Enabled = ipv6
	}

	if cfg.ID != "" {
		c.id = cfg.ID
	}

	if cfg.MetricNameRegex != "" {
		rx, err := regexp.Compile(cfg.MetricNameRegex)
		if err != nil {
			return nil, fmt.Errorf("%s compile metric_name_regex: %w", c.pkgID, err)
		}
		c.metricNameRegex = rx
	}

	if cfg.MetricNameChar != "" {
		c.metricNameChar = cfg.MetricNameChar
	}

	if cfg.RunTTL != "" {
		dur, err := time.ParseDuration(cfg.RunTTL)
		if err != nil {
			return nil, fmt.Errorf("%s parsing run_ttl: %w", c.pkgID, err)
		}
		c.runTTL = dur
	}

	return &c, nil
}

// Collect metrics from the wmi resource.
func (c *NetTCP) Collect(ctx context.Context) error {
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
	tagUnitsConnections := cgm.Tag{Category: "units", Value: "connections"}
	tagUnitsSegments := cgm.Tag{Category: "units", Value: "segments"}

	if c.ipv4Enabled {
		var dst []Win32_PerfRawData_Tcpip_TCPv4
		qry := wmi.CreateQuery(dst, "")
		if err := wmi.Query(qry, &dst); err != nil {
			c.logger.Error().Err(err).Str("query", qry).Msg("wmi query error")
			c.setStatus(metrics, err)
			return fmt.Errorf("wmi %s query: %w", c.pkgID, err)
		}

		if len(dst) > 1 {
			c.logger.Warn().Int("len", len(dst)).Msg("prot tcp4 metrics has more than one SET of enteries")
		}

		protoTag := cgm.Tag{Category: "network-proto", Value: "tcp4"}

		for _, item := range dst {
			if done(ctx) {
				return fmt.Errorf("context: %w", ctx.Err())
			}

			_ = c.addMetric(&metrics, "", "ConnectionFailures", metricType, item.ConnectionFailures, cgm.Tags{protoTag, tagUnitsConnections})
			_ = c.addMetric(&metrics, "", "ConnectionsActive", metricType, item.ConnectionsActive, cgm.Tags{protoTag, tagUnitsConnections})
			_ = c.addMetric(&metrics, "", "ConnectionsEstablished", metricType, item.ConnectionsEstablished, cgm.Tags{protoTag, tagUnitsConnections})
			_ = c.addMetric(&metrics, "", "ConnectionsPassive", metricType, item.ConnectionsPassive, cgm.Tags{protoTag, tagUnitsConnections})
			_ = c.addMetric(&metrics, "", "ConnectionsReset", metricType, item.ConnectionsReset, cgm.Tags{protoTag, tagUnitsConnections})
			_ = c.addMetric(&metrics, "", "SegmentsPersec", metricType, item.SegmentsPersec, cgm.Tags{protoTag, tagUnitsSegments})
			_ = c.addMetric(&metrics, "", "SegmentsReceivedPersec", metricType, item.SegmentsReceivedPersec, cgm.Tags{protoTag, tagUnitsSegments})
			_ = c.addMetric(&metrics, "", "SegmentsRetransmittedPersec", metricType, item.SegmentsRetransmittedPersec, cgm.Tags{protoTag, tagUnitsSegments})
			_ = c.addMetric(&metrics, "", "SegmentsSentPersec", metricType, item.SegmentsSentPersec, cgm.Tags{protoTag, tagUnitsSegments})
		}
	}

	if c.ipv6Enabled {
		var dst []Win32_PerfRawData_Tcpip_TCPv6
		qry := wmi.CreateQuery(dst, "")
		if err := wmi.Query(qry, &dst); err != nil {
			c.logger.Error().Err(err).Str("query", qry).Msg("wmi query error")
			c.setStatus(metrics, err)
			return fmt.Errorf("wmi %s query: %w", c.pkgID, err)
		}

		if len(dst) > 1 {
			c.logger.Warn().Int("len", len(dst)).Msg("prot tcp4 metrics has more than one SET of enteries")
		}

		protoTag := cgm.Tag{Category: "network-proto", Value: "tcp6"}

		for _, item := range dst {
			if done(ctx) {
				return fmt.Errorf("context: %w", ctx.Err())
			}

			_ = c.addMetric(&metrics, "", "ConnectionFailures", metricType, item.ConnectionFailures, cgm.Tags{protoTag, tagUnitsConnections})
			_ = c.addMetric(&metrics, "", "ConnectionsActive", metricType, item.ConnectionsActive, cgm.Tags{protoTag, tagUnitsConnections})
			_ = c.addMetric(&metrics, "", "ConnectionsEstablished", metricType, item.ConnectionsEstablished, cgm.Tags{protoTag, tagUnitsConnections})
			_ = c.addMetric(&metrics, "", "ConnectionsPassive", metricType, item.ConnectionsPassive, cgm.Tags{protoTag, tagUnitsConnections})
			_ = c.addMetric(&metrics, "", "ConnectionsReset", metricType, item.ConnectionsReset, cgm.Tags{protoTag, tagUnitsConnections})
			_ = c.addMetric(&metrics, "", "SegmentsPersec", metricType, item.SegmentsPersec, cgm.Tags{protoTag, tagUnitsSegments})
			_ = c.addMetric(&metrics, "", "SegmentsReceivedPersec", metricType, item.SegmentsReceivedPersec, cgm.Tags{protoTag, tagUnitsSegments})
			_ = c.addMetric(&metrics, "", "SegmentsRetransmittedPersec", metricType, item.SegmentsRetransmittedPersec, cgm.Tags{protoTag, tagUnitsSegments})
			_ = c.addMetric(&metrics, "", "SegmentsSentPersec", metricType, item.SegmentsSentPersec, cgm.Tags{protoTag, tagUnitsSegments})
		}
	}

	c.setStatus(metrics, nil)
	return nil
}
