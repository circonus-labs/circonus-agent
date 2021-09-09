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

// Win32_PerfRawData_Tcpip_NetworkInterface defines the metrics to collect
// https://technet.microsoft.com/en-us/security/aa394340(v=vs.80)
type Win32_PerfRawData_Tcpip_NetworkInterface struct { //nolint: golint
	Name                            string
	BytesReceivedPersec             uint64
	BytesSentPersec                 uint64
	BytesTotalPersec                uint64
	CurrentBandwidth                uint64
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

// NetInterface metrics from the Windows Management Interface (wmi).
type NetInterface struct {
	include *regexp.Regexp
	exclude *regexp.Regexp
	wmicommon
}

// netInterfaceOptions defines what elements can be overridden in a config file.
type netInterfaceOptions struct {
	ID              string `json:"id" toml:"id" yaml:"id"`
	IncludeRegex    string `json:"include_regex" toml:"include_regex" yaml:"include_regex"`
	ExcludeRegex    string `json:"exclude_regex" toml:"exclude_regex" yaml:"exclude_regex"`
	MetricNameRegex string `json:"metric_name_regex" toml:"metric_name_regex" yaml:"metric_name_regex"`
	MetricNameChar  string `json:"metric_name_char" toml:"metric_name_char" yaml:"metric_name_char"`
	RunTTL          string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

// NewNetInterfaceCollector creates new wmi collector.
func NewNetInterfaceCollector(cfgBaseName string) (collector.Collector, error) {
	c := NetInterface{}
	c.id = "network_interface"
	c.pkgID = pkgName + "." + c.id
	c.logger = log.With().Str("pkg", pkgName).Str("id", c.id).Logger()
	c.metricNameChar = defaultMetricChar
	c.metricNameRegex = defaultMetricNameRegex
	c.baseTags = tags.FromList(tags.GetBaseTags())

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
		return nil, fmt.Errorf("%s config: %w", c.pkgID, err)
	}

	c.logger.Debug().Interface("config", cfg).Msg("loaded config")

	// include regex
	if cfg.IncludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, cfg.IncludeRegex))
		if err != nil {
			return nil, fmt.Errorf("%s compile include rx: %w", c.pkgID, err)
		}
		c.include = rx
	}

	// exclude regex
	if cfg.ExcludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, cfg.ExcludeRegex))
		if err != nil {
			return nil, fmt.Errorf("%s compile exclude rx: %w", c.pkgID, err)
		}
		c.exclude = rx
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
func (c *NetInterface) Collect(ctx context.Context) error {
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
		c.logger.Error().Err(err).Str("query", qry).Msg("wmi query error")
		c.setStatus(metrics, err)
		return fmt.Errorf("wmi %s query: %w", c.pkgID, err)
	}

	metricType := "L"
	tagUnitsBytes := cgm.Tag{Category: "units", Value: "bytes"}
	tagUnitsBits := cgm.Tag{Category: "units", Value: "bits"}
	tagUnitsPackets := cgm.Tag{Category: "units", Value: "packets"}

	for _, ifMetrics := range dst {
		ifName := c.cleanName(ifMetrics.Name)
		if c.exclude.MatchString(ifName) || !c.include.MatchString(ifName) {
			continue
		}

		metricSuffix := ""
		if strings.Contains(ifMetrics.Name, totalName) {
			ifName = "all"
			metricSuffix = totalName
		}

		ifTag := cgm.Tag{Category: "network-interface", Value: ifName}

		_ = c.addMetric(&metrics, "", "BytesReceivedPersec"+metricSuffix, metricType, ifMetrics.BytesReceivedPersec, cgm.Tags{ifTag, tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "BytesSentPersec"+metricSuffix, metricType, ifMetrics.BytesSentPersec, cgm.Tags{ifTag, tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "BytesTotalPersec"+metricSuffix, metricType, ifMetrics.BytesTotalPersec, cgm.Tags{ifTag, tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "CurrentBandwidth"+metricSuffix, metricType, ifMetrics.CurrentBandwidth, cgm.Tags{ifTag, tagUnitsBits})
		_ = c.addMetric(&metrics, "", "OffloadedConnections"+metricSuffix, metricType, ifMetrics.OffloadedConnections, cgm.Tags{ifTag})
		_ = c.addMetric(&metrics, "", "OutputQueueLength"+metricSuffix, metricType, ifMetrics.OutputQueueLength, cgm.Tags{ifTag})
		_ = c.addMetric(&metrics, "", "PacketsOutboundDiscarded"+metricSuffix, metricType, ifMetrics.PacketsOutboundDiscarded, cgm.Tags{ifTag, tagUnitsPackets})
		_ = c.addMetric(&metrics, "", "PacketsOutboundErrors"+metricSuffix, metricType, ifMetrics.PacketsOutboundErrors, cgm.Tags{ifTag, tagUnitsPackets})
		_ = c.addMetric(&metrics, "", "PacketsPersec"+metricSuffix, metricType, ifMetrics.PacketsPersec, cgm.Tags{ifTag, tagUnitsPackets})
		_ = c.addMetric(&metrics, "", "PacketsReceivedDiscarded"+metricSuffix, metricType, ifMetrics.PacketsReceivedDiscarded, cgm.Tags{ifTag, tagUnitsPackets})
		_ = c.addMetric(&metrics, "", "PacketsReceivedErrors"+metricSuffix, metricType, ifMetrics.PacketsReceivedErrors, cgm.Tags{ifTag, tagUnitsPackets})
		_ = c.addMetric(&metrics, "", "PacketsReceivedNonUnicastPersec"+metricSuffix, metricType, ifMetrics.PacketsReceivedNonUnicastPersec, cgm.Tags{ifTag, tagUnitsPackets})
		_ = c.addMetric(&metrics, "", "PacketsReceivedPersec"+metricSuffix, metricType, ifMetrics.PacketsReceivedPersec, cgm.Tags{ifTag, tagUnitsPackets})
		_ = c.addMetric(&metrics, "", "PacketsReceivedUnicastPersec"+metricSuffix, metricType, ifMetrics.PacketsReceivedUnicastPersec, cgm.Tags{ifTag, tagUnitsPackets})
		_ = c.addMetric(&metrics, "", "PacketsReceivedUnknown"+metricSuffix, metricType, ifMetrics.PacketsReceivedUnknown, cgm.Tags{ifTag, tagUnitsPackets})
		_ = c.addMetric(&metrics, "", "PacketsSentNonUnicastPersec"+metricSuffix, metricType, ifMetrics.PacketsSentNonUnicastPersec, cgm.Tags{ifTag, tagUnitsPackets})
		_ = c.addMetric(&metrics, "", "PacketsSentPersec"+metricSuffix, metricType, ifMetrics.PacketsSentPersec, cgm.Tags{ifTag, tagUnitsPackets})
		_ = c.addMetric(&metrics, "", "PacketsSentUnicastPersec"+metricSuffix, metricType, ifMetrics.PacketsSentUnicastPersec, cgm.Tags{ifTag, tagUnitsPackets})
		_ = c.addMetric(&metrics, "", "TCPActiveRSCConnections"+metricSuffix, metricType, ifMetrics.TCPActiveRSCConnections, cgm.Tags{ifTag})
		_ = c.addMetric(&metrics, "", "TCPRSCAveragePacketSize"+metricSuffix, metricType, ifMetrics.TCPRSCAveragePacketSize, cgm.Tags{ifTag, tagUnitsBytes})
		_ = c.addMetric(&metrics, "", "TCPRSCCoalescedPacketsPersec"+metricSuffix, metricType, ifMetrics.TCPRSCCoalescedPacketsPersec, cgm.Tags{ifTag, tagUnitsPackets})
		_ = c.addMetric(&metrics, "", "TCPRSCExceptionsPersec"+metricSuffix, metricType, ifMetrics.TCPRSCExceptionsPersec, cgm.Tags{ifTag})
	}

	c.setStatus(metrics, nil)
	return nil
}
