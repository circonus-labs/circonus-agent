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

// Win32_PerfRawData_Tcpip_IPv4 defines the metrics to collect.
type Win32_PerfRawData_Tcpip_IPv4 struct { //nolint: revive
	DatagramsForwardedPersec         uint32
	DatagramsOutboundDiscarded       uint32
	DatagramsOutboundNoRoute         uint32
	DatagramsPersec                  uint32
	DatagramsReceivedAddressErrors   uint32
	DatagramsReceivedDeliveredPersec uint32
	DatagramsReceivedDiscarded       uint32
	DatagramsReceivedHeaderErrors    uint32
	DatagramsReceivedPersec          uint32
	DatagramsReceivedUnknownProtocol uint32
	DatagramsSentPersec              uint32
	FragmentationFailures            uint32
	FragmentedDatagramsPersec        uint32
	FragmentReassemblyFailures       uint32
	FragmentsCreatedPersec           uint32
	FragmentsReassembledPersec       uint32
	FragmentsReceivedPersec          uint32
}

// Win32_PerfRawData_Tcpip_IPv6 defines the metrics to collect.
type Win32_PerfRawData_Tcpip_IPv6 struct { //nolint: revive
	DatagramsForwardedPersec         uint32
	DatagramsOutboundDiscarded       uint32
	DatagramsOutboundNoRoute         uint32
	DatagramsPersec                  uint32
	DatagramsReceivedAddressErrors   uint32
	DatagramsReceivedDeliveredPersec uint32
	DatagramsReceivedDiscarded       uint32
	DatagramsReceivedHeaderErrors    uint32
	DatagramsReceivedPersec          uint32
	DatagramsReceivedUnknownProtocol uint32
	DatagramsSentPersec              uint32
	FragmentationFailures            uint32
	FragmentedDatagramsPersec        uint32
	FragmentReassemblyFailures       uint32
	FragmentsCreatedPersec           uint32
	FragmentsReassembledPersec       uint32
	FragmentsReceivedPersec          uint32
}

// NetIP metrics from the Windows Management Interface (wmi).
type NetIP struct {
	wmicommon
	ipv4Enabled bool
	ipv6Enabled bool
}

// NetIPOptions defines what elements can be overridden in a config file.
type NetIPOptions struct {
	ID              string `json:"id" toml:"id" yaml:"id"`
	MetricNameRegex string `json:"metric_name_regex" toml:"metric_name_regex" yaml:"metric_name_regex"`
	MetricNameChar  string `json:"metric_name_char" toml:"metric_name_char" yaml:"metric_name_char"`
	RunTTL          string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
	EnableIPv4      string `json:"enable_ipv4" toml:"enable_ipv4" yaml:"enable_ipv4"`
	EnableIPv6      string `json:"enable_ipv6" toml:"enable_ipv6" yaml:"enable_ipv6"`
}

// NewNetIPCollector creates new wmi collector.
func NewNetIPCollector(cfgBaseName string) (collector.Collector, error) {
	c := NetIP{}
	c.id = "net_ip"
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

	var cfg NetIPOptions
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
			return nil, fmt.Errorf("%s compile metric name rx: %w", c.pkgID, err)
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
func (c *NetIP) Collect(ctx context.Context) error {
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
	tagUnitsFragments := cgm.Tag{Category: "units", Value: "fragments"}

	if c.ipv4Enabled {
		var dst []Win32_PerfRawData_Tcpip_IPv4
		qry := wmi.CreateQuery(dst, "")
		if err := wmi.Query(qry, &dst); err != nil {
			c.logger.Error().Err(err).Str("query", qry).Msg("wmi query error")
			c.setStatus(metrics, err)
			return fmt.Errorf("wmi %s query: %w", c.pkgID, err)
		}

		if len(dst) > 1 {
			c.logger.Warn().Int("len", len(dst)).Msg("prot ip4 metrics has more than one SET of enteries")
		}

		protoTag := cgm.Tag{Category: "network-proto", Value: "ip4"}

		for _, item := range dst {
			if done(ctx) {
				return fmt.Errorf("context: %w", ctx.Err())
			}

			_ = c.addMetric(&metrics, "", "DatagramsForwardedPersec", metricType, item.DatagramsForwardedPersec, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsOutboundDiscarded", metricType, item.DatagramsOutboundDiscarded, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsOutboundNoRoute", metricType, item.DatagramsOutboundNoRoute, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsPersec", metricType, item.DatagramsPersec, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsReceivedAddressErrors", metricType, item.DatagramsReceivedAddressErrors, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsReceivedDeliveredPersec", metricType, item.DatagramsReceivedDeliveredPersec, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsReceivedDiscarded", metricType, item.DatagramsReceivedDiscarded, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsReceivedHeaderErrors", metricType, item.DatagramsReceivedHeaderErrors, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsReceivedPersec", metricType, item.DatagramsReceivedPersec, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsReceivedUnknownProtocol", metricType, item.DatagramsReceivedUnknownProtocol, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsSentPersec", metricType, item.DatagramsSentPersec, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "FragmentationFailures", metricType, item.FragmentationFailures, cgm.Tags{protoTag, tagUnitsFragments})
			_ = c.addMetric(&metrics, "", "FragmentedDatagramsPersec", metricType, item.FragmentedDatagramsPersec, cgm.Tags{protoTag, tagUnitsFragments})
			_ = c.addMetric(&metrics, "", "FragmentReassemblyFailures", metricType, item.FragmentReassemblyFailures, cgm.Tags{protoTag, tagUnitsFragments})
			_ = c.addMetric(&metrics, "", "FragmentsCreatedPersec", metricType, item.FragmentsCreatedPersec, cgm.Tags{protoTag, tagUnitsFragments})
			_ = c.addMetric(&metrics, "", "FragmentsReassembledPersec", metricType, item.FragmentsReassembledPersec, cgm.Tags{protoTag, tagUnitsFragments})
			_ = c.addMetric(&metrics, "", "FragmentsReceivedPersec", metricType, item.FragmentsReceivedPersec, cgm.Tags{protoTag, tagUnitsFragments})
		}
	}

	if c.ipv6Enabled {
		var dst []Win32_PerfRawData_Tcpip_IPv6
		qry := wmi.CreateQuery(dst, "")
		if err := wmi.Query(qry, &dst); err != nil {
			c.logger.Error().Err(err).Str("query", qry).Msg("wmi query error")
			c.setStatus(metrics, err)
			return fmt.Errorf("wmi %s query: %w", c.pkgID, err)
		}

		if len(dst) > 1 {
			c.logger.Warn().Int("len", len(dst)).Msg("prot ip6 metrics has more than one SET of enteries")
		}

		protoTag := cgm.Tag{Category: "network-proto", Value: "ip6"}

		for _, item := range dst {
			if done(ctx) {
				return fmt.Errorf("context: %w", ctx.Err())
			}

			_ = c.addMetric(&metrics, "", "DatagramsForwardedPersec", metricType, item.DatagramsForwardedPersec, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsOutboundDiscarded", metricType, item.DatagramsOutboundDiscarded, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsOutboundNoRoute", metricType, item.DatagramsOutboundNoRoute, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsPersec", metricType, item.DatagramsPersec, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsReceivedAddressErrors", metricType, item.DatagramsReceivedAddressErrors, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsReceivedDeliveredPersec", metricType, item.DatagramsReceivedDeliveredPersec, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsReceivedDiscarded", metricType, item.DatagramsReceivedDiscarded, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsReceivedHeaderErrors", metricType, item.DatagramsReceivedHeaderErrors, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsReceivedPersec", metricType, item.DatagramsReceivedPersec, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsReceivedUnknownProtocol", metricType, item.DatagramsReceivedUnknownProtocol, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "DatagramsSentPersec", metricType, item.DatagramsSentPersec, cgm.Tags{protoTag, tagUnitsDatagrams})
			_ = c.addMetric(&metrics, "", "FragmentationFailures", metricType, item.FragmentationFailures, cgm.Tags{protoTag, tagUnitsFragments})
			_ = c.addMetric(&metrics, "", "FragmentedDatagramsPersec", metricType, item.FragmentedDatagramsPersec, cgm.Tags{protoTag, tagUnitsFragments})
			_ = c.addMetric(&metrics, "", "FragmentReassemblyFailures", metricType, item.FragmentReassemblyFailures, cgm.Tags{protoTag, tagUnitsFragments})
			_ = c.addMetric(&metrics, "", "FragmentsCreatedPersec", metricType, item.FragmentsCreatedPersec, cgm.Tags{protoTag, tagUnitsFragments})
			_ = c.addMetric(&metrics, "", "FragmentsReassembledPersec", metricType, item.FragmentsReassembledPersec, cgm.Tags{protoTag, tagUnitsFragments})
			_ = c.addMetric(&metrics, "", "FragmentsReceivedPersec", metricType, item.FragmentsReceivedPersec, cgm.Tags{protoTag, tagUnitsFragments})
		}
	}

	c.setStatus(metrics, nil)
	return nil
}
