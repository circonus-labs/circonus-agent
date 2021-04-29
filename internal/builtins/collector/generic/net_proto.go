// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/net"
)

// Proto metrics.
type Proto struct {
	protocols []string
	gencommon
}

// protoOptions defines what elements can be overridden in a config file.
type protoOptions struct {
	// common
	ID     string `json:"id" toml:"id" yaml:"id"`
	RunTTL string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`

	// collector specific
	Protocols []string `json:"protocols" toml:"protocols" yaml:"protocols"` // default: empty (equates to all: ip,icmp,icmpmsg,tcp,udp,udplite)
}

var (
	errNoProtoMetrics = fmt.Errorf("no network protocol metrics available")
)

// NewNetProtoCollector creates new psutils collector.
func NewNetProtoCollector(cfgBaseName string, parentLogger zerolog.Logger) (collector.Collector, error) {
	c := Proto{}
	c.id = NameProto
	c.pkgID = PackageName + "." + c.id
	c.logger = parentLogger.With().Str("id", c.id).Logger()
	c.baseTags = tags.FromList(tags.GetBaseTags())

	var opts protoOptions
	err := config.LoadConfigFile(cfgBaseName, &opts)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, fmt.Errorf("%s config: %w", c.pkgID, err)
	}

	c.logger.Debug().Str("base", cfgBaseName).Interface("config", opts).Msg("loaded config")

	if len(opts.Protocols) > 0 {
		c.protocols = opts.Protocols
	}

	if opts.ID != "" {
		c.id = opts.ID
	}

	if opts.RunTTL != "" {
		dur, err := time.ParseDuration(opts.RunTTL)
		if err != nil {
			return nil, fmt.Errorf("%s parsing run_ttl: %w", c.pkgID, err)
		}
		c.runTTL = dur
	}

	return &c, nil
}

// Collect metrics.
func (c *Proto) Collect(ctx context.Context) error {
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

	//
	// NOTE: gopsutil does not currently offer IPv6 metrics
	//       it only pulls from /proc/net/snmp not /proc/net/snmp6
	//

	metrics := cgm.Metrics{}
	counters, err := net.ProtoCounters(c.protocols)
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting network protocol metrics")
		c.setStatus(metrics, nil)
		return nil
	}

	if len(counters) == 0 {
		return errNoProtoMetrics
	}

	if runtime.GOOS == "linux" {
		for _, counter := range counters {
			protoTags := tags.Tags{tags.Tag{Category: "protocol", Value: counter.Protocol}}
			for name, val := range counter.Stats {
				switch counter.Protocol {
				case "ip":
					c.emitIPMetric(&metrics, counter.Protocol, name, val, protoTags)
				case "icmp":
					c.emitICMPMetric(&metrics, counter.Protocol, name, val, protoTags)
				case "icmpmsg":
					c.emitICMPMsgMetric(&metrics, counter.Protocol, name, val, protoTags)
				case "tcp":
					c.emitTCPMetric(&metrics, counter.Protocol, name, val, protoTags)
				case "udp":
					c.emitUDPMetric(&metrics, counter.Protocol, name, val, protoTags)
				case "udplite":
					c.emitUDPLiteMetric(&metrics, counter.Protocol, name, val, protoTags)
				default:
					// NOTE: output msg for each metric, so support can be added without having to
					//       set up an environment that can produce the metric(s)
					c.logger.Warn().Str("protocol", counter.Protocol).Str("metric", name).Msg("unknown protocol, missing units info")
					_ = c.addMetric(&metrics, name, "L", val, protoTags)
				}
			}
		}

		c.setStatus(metrics, nil)
		return nil
	}

	for _, counter := range counters {
		protoTags := tags.Tags{tags.Tag{Category: "protocol", Value: counter.Protocol}}
		for name, val := range counter.Stats {
			// NOTE: output msg for EACH proto/metric so a log can be supplied.
			//       support can then be added from the log vs having to run the OS itself.
			c.logger.Warn().
				Str("os_type", runtime.GOOS).
				Str("protocol", counter.Protocol).
				Str("metric", name).
				Msg("unknown os type supported, missing units info")
			_ = c.addMetric(&metrics, name, "L", val, protoTags)
		}
	}

	c.setStatus(metrics, nil)
	return nil
}

const (
	// repeated metric names.
	metricInCsumErrors = "InCsumErrors"
	metricInErrors     = "InErrors"
	metricOutDatagrams = "OutDatagrams"
	metricInDatagrams  = "InDatagrams"
	metricSndbufErrors = "SndbufErrors"
	metricRcvbufErrors = "RcvbufErrors"
	metricNoPorts      = "NoPorts"
	defaultMetricType  = "l"
)

func (c *Proto) emitIPMetric(metrics *cgm.Metrics, proto, name string, val int64, protoTags tags.Tags) {
	// https://sourceforge.net/p/net-tools/code/ci/master/tree/statistics.c#l56
	// https://tools.ietf.org/html/rfc1213

	tagUnitsSeconds := tags.Tag{Category: "units", Value: "seconds"}
	tagUnitsDatagrams := tags.Tag{Category: "units", Value: "datagrams"}
	tagUnitsFragments := tags.Tag{Category: "units", Value: "fragments"}
	tagUnitsPackets := tags.Tag{Category: "units", Value: "packets"}
	tagUnitsRequests := tags.Tag{Category: "units", Value: "requests"}

	var tagList tags.Tags
	tagList = append(tagList, protoTags...)

	switch name {
	case "ReasmTimeout":
		tagList = append(tagList, tagUnitsSeconds)
	case "DefaultTTL":
		tagList = append(tagList, tagUnitsSeconds)
	case "ForwDatagrams":
		tagList = append(tagList, tagUnitsDatagrams)
	case "Forwarding":
		// it's a setting, not really a metric, no units.
		// 1 - acting as gateway
		// 2 - NOT acting as gateway
	case "FragCreates":
		tagList = append(tagList, tagUnitsFragments)
	case "FragFails":
		tagList = append(tagList, tagUnitsFragments)
	case "FragOKs":
		tagList = append(tagList, tagUnitsFragments)
	case "InAddrErrors":
		// no units
	case "InDelivers":
		tagList = append(tagList, tagUnitsPackets)
	case "InDiscards":
		tagList = append(tagList, tagUnitsPackets)
	case "InHdrErrors":
		tagList = append(tagList, tagUnitsPackets)
	case "InReceives":
		tagList = append(tagList, tagUnitsPackets)
	case "InUnknownProtos":
		tagList = append(tagList, tagUnitsPackets)
	case "OutDiscards":
		tagList = append(tagList, tagUnitsPackets)
	case "OutNoRoutes":
		tagList = append(tagList, tagUnitsPackets)
	case "ReasmFails":
		tagList = append(tagList, tagUnitsPackets)
	case "ReasmOKs":
		tagList = append(tagList, tagUnitsPackets)
	case "ReasmReqds":
		tagList = append(tagList, tagUnitsPackets)
	case "OutRequests":
		tagList = append(tagList, tagUnitsRequests)
	default:
		c.logger.Warn().Str("protocol", proto).Str("metric", name).Msg("unrecognized metric, no units")
	}

	_ = c.addMetric(metrics, name, defaultMetricType, val, tagList)
}

func (c *Proto) emitICMPMetric(metrics *cgm.Metrics, proto, name string, val int64, protoTags tags.Tags) {
	// https://sourceforge.net/p/net-tools/code/ci/master/tree/statistics.c#l105
	// https://tools.ietf.org/html/rfc1213

	tagUnitsResponses := tags.Tag{Category: "units", Value: "responses"}
	tagUnitsRequests := tags.Tag{Category: "units", Value: "requests"}
	tagUnitsMessages := tags.Tag{Category: "units", Value: "messages"}
	tagUnitsRedirects := tags.Tag{Category: "units", Value: "redirects"}

	var tagList tags.Tags
	tagList = append(tagList, protoTags...)
	switch name {
	case "OutTimestampReps":
		tagList = append(tagList, tagUnitsResponses)
	case "OutEchoReps":
		tagList = append(tagList, tagUnitsResponses)
	case "OutAddrMaskReps":
		tagList = append(tagList, tagUnitsResponses)
	case "InTimestampReps":
		tagList = append(tagList, tagUnitsResponses)
	case "InEchoReps":
		tagList = append(tagList, tagUnitsResponses)
	case "InAddrMaskReps":
		tagList = append(tagList, tagUnitsResponses)
	case "OutTimestamps":
		tagList = append(tagList, tagUnitsRequests)
	case "OutEchos":
		tagList = append(tagList, tagUnitsRequests)
	case "OutAddrMasks":
		tagList = append(tagList, tagUnitsRequests)
	case "InTimestamps":
		tagList = append(tagList, tagUnitsRequests)
	case "InEchos":
		tagList = append(tagList, tagUnitsRequests)
	case "InAddrMasks":
		tagList = append(tagList, tagUnitsRequests)
	case "OutDestUnreachs":
		// no units
	case "InDestUnreachs":
		// no units
	case "OutParmProbs":
		// no units
	case "InParmProbs":
		// no units
	case "OutErrors":
		// no units
	case metricInCsumErrors:
		// no units
	case metricInErrors:
		// no units
	case "OutMsgs":
		tagList = append(tagList, tagUnitsMessages)
	case "InMsgs":
		tagList = append(tagList, tagUnitsMessages)
	case "OutRedirects":
		tagList = append(tagList, tagUnitsRedirects)
	case "InRedirects":
		tagList = append(tagList, tagUnitsRedirects)
	case "OutSrcQuenchs":
		// no units
	case "InSrcQuenchs":
		// no units
	case "OutTimeExcds":
		// no units
	case "InTimeExcds":
		// no units
	default:
		c.logger.Warn().Str("protocol", proto).Str("metric", name).Msg("unrecognized metric, no units")
	}
	_ = c.addMetric(metrics, name, defaultMetricType, val, tagList)
}

func (c *Proto) emitICMPMsgMetric(metrics *cgm.Metrics, proto, name string, val int64, protoTags tags.Tags) {
	_ = proto // just so it's used and the func call signatures stay consistent

	// possible message types:
	// https://www.iana.org/assignments/icmp-parameters/icmp-parameters.xhtml

	tagList := tags.Tags{tags.Tag{Category: "units", Value: "messages"}}
	tagList = append(tagList, protoTags...)

	_ = c.addMetric(metrics, name, defaultMetricType, val, tagList)
}

func (c *Proto) emitTCPMetric(metrics *cgm.Metrics, proto, name string, val int64, protoTags tags.Tags) {
	// https://sourceforge.net/p/net-tools/code/ci/master/tree/statistics.c#l170
	// https://tools.ietf.org/html/rfc1213

	tagUnitsConnections := tags.Tag{Category: "units", Value: "connections"}
	tagUnitsSegments := tags.Tag{Category: "units", Value: "segments"}
	tagUnitsResets := tags.Tag{Category: "units", Value: "resets"}
	tagUnitsMilliseconds := tags.Tag{Category: "units", Value: "milliseconds"}

	var tagList tags.Tags
	tagList = append(tagList, protoTags...)
	switch name {
	case "AttemptFails":
		tagList = append(tagList, tagUnitsConnections)
	case "MaxConn":
		tagList = append(tagList, tagUnitsConnections)
	case "PassiveOpens":
		tagList = append(tagList, tagUnitsConnections)
	case "CurrEstab":
		tagList = append(tagList, tagUnitsConnections)
	case "ActiveOpens":
		tagList = append(tagList, tagUnitsConnections)
	case "OutSegs":
		tagList = append(tagList, tagUnitsSegments)
	case "InSegs":
		tagList = append(tagList, tagUnitsSegments)
	case "RetransSegs":
		tagList = append(tagList, tagUnitsSegments)
	case metricInCsumErrors:
		// no units
	case "InErrs":
		// no units
	case "EstabResets":
		tagList = append(tagList, tagUnitsResets)
	case "OutRsts":
		tagList = append(tagList, tagUnitsResets)
	case "RtoAlgorithm":
		// it's a setting, not a metric, no units
		// 1 none of the following
		// 2 constant rto
		// 3 mil-std-1778
		// 4 van jacobson's algorithm
	case "RtoMax":
		tagList = append(tagList, tagUnitsMilliseconds)
	case "RtoMin":
		tagList = append(tagList, tagUnitsMilliseconds)
	default:
		c.logger.Warn().Str("protocol", proto).Str("metric", name).Msg("unrecognized metric, no units")
	}
	_ = c.addMetric(metrics, name, defaultMetricType, val, tagList)
}

func (c *Proto) emitUDPMetric(metrics *cgm.Metrics, proto, name string, val int64, protoTags tags.Tags) {
	// https://sourceforge.net/p/net-tools/code/ci/master/tree/statistics.c#l188
	// https://tools.ietf.org/html/rfc1213

	tagUnitsDatagrams := tags.Tag{Category: "units", Value: "datagrams"}

	var tagList tags.Tags
	tagList = append(tagList, protoTags...)
	switch name {
	case metricOutDatagrams:
		tagList = append(tagList, tagUnitsDatagrams)
	case metricInDatagrams:
		tagList = append(tagList, tagUnitsDatagrams)
	case metricInCsumErrors:
		// no units
	case metricSndbufErrors:
		// no units
	case metricRcvbufErrors:
		// no units
	case metricNoPorts:
		// no units
	case metricInErrors:
		// no units
	default:
		c.logger.Warn().Str("protocol", proto).Str("metric", name).Msg("unrecognized metric, no units")
	}
	_ = c.addMetric(metrics, name, defaultMetricType, val, tagList)
}

func (c *Proto) emitUDPLiteMetric(metrics *cgm.Metrics, proto, name string, val int64, protoTags tags.Tags) {
	// same names as UDP...

	tagUnitsDatagrams := tags.Tag{Category: "units", Value: "datagrams"}

	var tagList tags.Tags
	tagList = append(tagList, protoTags...)
	switch name {
	case metricOutDatagrams:
		tagList = append(tagList, tagUnitsDatagrams)
	case metricInDatagrams:
		tagList = append(tagList, tagUnitsDatagrams)
	case metricInCsumErrors:
		// no units
	case metricSndbufErrors:
		// no units
	case metricRcvbufErrors:
		// no units
	case metricNoPorts:
		// no units
	case metricInErrors:
		// no units
	default:
		c.logger.Warn().Str("protocol", proto).Str("metric", name).Msg("unrecognized metric, no units")
	}
	_ = c.addMetric(metrics, name, defaultMetricType, val, tagList)
}
