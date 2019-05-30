// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
	"runtime"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/net"
)

// Proto metrics
type Proto struct {
	gencommon
	protocols []string
}

// protoOptions defines what elements can be overridden in a config file
type protoOptions struct {
	// common
	ID     string `json:"id" toml:"id" yaml:"id"`
	RunTTL string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`

	// collector specific
	Protocols []string `json:"protocols" toml:"protocols" yaml:"protocols"` // default: empty (equates to all: ip,icmp,icmpmsg,tcp,udp,udplite)
}

// NewNetProtoCollector creates new psutils collector
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
		return nil, errors.Wrapf(err, "%s config", c.pkgID)
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
			return nil, errors.Wrapf(err, "%s parsing run_ttl", c.pkgID)
		}
		c.runTTL = dur
	}

	return &c, nil
}

// Collect metrics
func (c *Proto) Collect() error {
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

	moduleTags := tags.Tags{
		tags.Tag{Category: release.NAME + "-module", Value: c.id},
	}

	//
	// NOTE: gopsutil does not currently offer IPv6 metrics
	//       it only pulls from /proc/net/snmp not /proc/net/snmp6
	//

	metrics := cgm.Metrics{}
	counters, err := net.ProtoCounters(c.protocols)
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting network protocol metrics")
	} else {
		if len(counters) == 0 {
			return errors.New("no network protocol metrics available")
		}
		if runtime.GOOS == "linux" {
			for _, counter := range counters {
				var protoTags tags.Tags
				protoTags = append(protoTags, moduleTags...)
				protoTags = append(protoTags, tags.Tag{Category: "protocol", Value: counter.Protocol})
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
						c.logger.Warn().Str("protocol", counter.Protocol).Str("metric", name).Msg("unknown protocol, missing units info")
						_ = c.addMetric(&metrics, name, "L", val, protoTags)
					}
				}
			}
		} else {
			for _, counter := range counters {
				var protoTags tags.Tags
				protoTags = append(protoTags, moduleTags...)
				protoTags = append(protoTags, tags.Tag{Category: "protocol", Value: counter.Protocol})
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
		}
	}

	c.setStatus(metrics, nil)
	return nil
}

const (
	// repeated metric names
	metricInCsumErrors = "InCsumErrors"
	metricInErrors     = "InErrors"
	metricOutDatagrams = "OutDatagrams"
	metricInDatagrams  = "InDatagrams"
	metricSndbufErrors = "SndbufErrors"
	metricRcvbufErrors = "RcvbufErrors"
	metricNoPorts      = "NoPorts"
)

func (c *Proto) emitIPMetric(metrics *cgm.Metrics, proto, name string, val int64, protoTags tags.Tags) {
	// https://sourceforge.net/p/net-tools/code/ci/master/tree/statistics.c#l56
	// https://tools.ietf.org/html/rfc1213
	var tagList tags.Tags
	tagList = append(tagList, protoTags...)
	switch name {
	case "ReasmTimeout":
		fallthrough
	case "DefaultTTL":
		tagList = append(tagList, tags.Tag{Category: "units", Value: "seconds"})
	case "ForwDatagrams":
		tagList = append(tagList, tags.Tag{Category: "units", Value: "datagrams"})
	case "Forwarding":
		// 1 - acting as gateway
		// 2 - NOT acting as gateway
		tagList = append(tagList, tags.Tag{Category: "units", Value: "forwarding"})
	case "FragCreates":
		fallthrough
	case "FragFails":
		fallthrough
	case "FragOKs":
		tagList = append(tagList, tags.Tag{Category: "units", Value: "fragments"})
	case "InAddrErrors":
		tagList = append(tagList, tags.Tag{Category: "units", Value: "errors"})
	case "InDelivers":
		fallthrough
	case "InDiscards":
		fallthrough
	case "InHdrErrors":
		fallthrough
	case "InReceives":
		fallthrough
	case "InUnknownProtos":
		fallthrough
	case "OutDiscards":
		fallthrough
	case "OutNoRoutes":
		fallthrough
	case "ReasmFails":
		fallthrough
	case "ReasmOKs":
		fallthrough
	case "ReasmReqds":
		tagList = append(tagList, tags.Tag{Category: "units", Value: "packets"})
	case "OutRequests":
		tagList = append(tagList, tags.Tag{Category: "units", Value: "requests"})
	default:
		c.logger.Warn().Str("protocol", proto).Str("metric", name).Msg("unrecognized metric, no units")
	}
	_ = c.addMetric(metrics, name, "L", val, tagList)
}

func (c *Proto) emitICMPMetric(metrics *cgm.Metrics, proto, name string, val int64, protoTags tags.Tags) {
	// https://sourceforge.net/p/net-tools/code/ci/master/tree/statistics.c#l105
	// https://tools.ietf.org/html/rfc1213
	var tagList tags.Tags
	tagList = append(tagList, protoTags...)
	switch name {
	case "OutTimestampReps":
		fallthrough
	case "OutEchoReps":
		fallthrough
	case "OutAddrMaskReps":
		fallthrough
	case "InTimestampReps":
		fallthrough
	case "InEchoReps":
		fallthrough
	case "InAddrMaskReps":
		tagList = append(tagList, tags.Tag{Category: "units", Value: "replies"})
	case "OutTimestamps":
		fallthrough
	case "OutEchos":
		fallthrough
	case "OutAddrMasks":
		fallthrough
	case "InTimestamps":
		fallthrough
	case "InEchos":
		fallthrough
	case "InAddrMasks":
		tagList = append(tagList, tags.Tag{Category: "units", Value: "requests"})
	case "OutDestUnreachs":
		fallthrough
	case "InDestUnreachs":
		tagList = append(tagList, tags.Tag{Category: "units", Value: "unreachable"})
	case "OutParmProbs":
		fallthrough
	case "InParmProbs":
		fallthrough
	case "OutErrors":
		fallthrough
	case metricInCsumErrors:
		fallthrough
	case metricInErrors:
		tagList = append(tagList, tags.Tag{Category: "units", Value: "errors"})
	case "OutMsgs":
		fallthrough
	case "InMsgs":
		tagList = append(tagList, tags.Tag{Category: "units", Value: "messages"})
	case "OutRedirects":
		fallthrough
	case "InRedirects":
		tagList = append(tagList, tags.Tag{Category: "units", Value: "redirects"})
	case "OutSrcQuenchs":
		fallthrough
	case "InSrcQuenchs":
		tagList = append(tagList, tags.Tag{Category: "units", Value: "quenches"})
	case "OutTimeExcds":
		fallthrough
	case "InTimeExcds":
		tagList = append(tagList, tags.Tag{Category: "units", Value: "timeout"})
	default:
		c.logger.Warn().Str("protocol", proto).Str("metric", name).Msg("unrecognized metric, no units")
	}
	_ = c.addMetric(metrics, name, "L", val, tagList)
}

func (c *Proto) emitICMPMsgMetric(metrics *cgm.Metrics, proto, name string, val int64, protoTags tags.Tags) {
	// possible message types:
	// https://www.iana.org/assignments/icmp-parameters/icmp-parameters.xhtml
	var tagList tags.Tags
	tagList = append(tagList, protoTags...)
	tagList = append(tagList, tags.Tag{Category: "units", Value: "messages"})
	_ = c.addMetric(metrics, name, "L", val, tagList)
}

func (c *Proto) emitTCPMetric(metrics *cgm.Metrics, proto, name string, val int64, protoTags tags.Tags) {
	// https://sourceforge.net/p/net-tools/code/ci/master/tree/statistics.c#l170
	// https://tools.ietf.org/html/rfc1213
	var tagList tags.Tags
	tagList = append(tagList, protoTags...)
	switch name {
	case "MaxConn":
		fallthrough
	case "PassiveOpens":
		fallthrough
	case "CurrEstab":
		fallthrough
	case "ActiveOpens":
		tagList = append(tagList, tags.Tag{Category: "units", Value: "connections"})
	case "OutSegs":
		fallthrough
	case "InSegs":
		fallthrough
	case "RetransSegs":
		tagList = append(tagList, tags.Tag{Category: "units", Value: "segments"})
	case "AttemptFails":
		fallthrough
	case metricInCsumErrors:
		fallthrough
	case "InErrs":
		tagList = append(tagList, tags.Tag{Category: "units", Value: "errors"})
	case "EstabResets":
		fallthrough
	case "OutRsts":
		tagList = append(tagList, tags.Tag{Category: "units", Value: "resets"})
	case "RtoAlgorithm":
		// 1 none of the following
		// 2 constant rto
		// 3 mil-std-1778
		// 4 van jacobson's algorithm
		tagList = append(tagList, tags.Tag{Category: "units", Value: "algorithm"})
	case "RtoMax":
		fallthrough
	case "RtoMin":
		tagList = append(tagList, tags.Tag{Category: "units", Value: "milliseconds"})
	default:
		c.logger.Warn().Str("protocol", proto).Str("metric", name).Msg("unrecognized metric, no units")
	}
	_ = c.addMetric(metrics, name, "L", val, tagList)
}

func (c *Proto) emitUDPMetric(metrics *cgm.Metrics, proto, name string, val int64, protoTags tags.Tags) {
	// https://sourceforge.net/p/net-tools/code/ci/master/tree/statistics.c#l188
	// https://tools.ietf.org/html/rfc1213
	var tagList tags.Tags
	tagList = append(tagList, protoTags...)
	switch name {
	case metricOutDatagrams:
		fallthrough
	case metricInDatagrams:
		tagList = append(tagList, tags.Tag{Category: "units", Value: "datagrams"})
	case metricInCsumErrors:
		fallthrough
	case metricSndbufErrors:
		fallthrough
	case metricRcvbufErrors:
		fallthrough
	case metricNoPorts:
		fallthrough
	case metricInErrors:
		tagList = append(tagList, tags.Tag{Category: "units", Value: "errors"})
	default:
		c.logger.Warn().Str("protocol", proto).Str("metric", name).Msg("unrecognized metric, no units")
	}
	_ = c.addMetric(metrics, name, "L", val, tagList)
}

func (c *Proto) emitUDPLiteMetric(metrics *cgm.Metrics, proto, name string, val int64, protoTags tags.Tags) {
	// same names as UDP...
	var tagList tags.Tags
	tagList = append(tagList, protoTags...)
	switch name {
	case metricOutDatagrams:
		fallthrough
	case metricInDatagrams:
		tagList = append(tagList, tags.Tag{Category: "units", Value: "datagrams"})
	case metricInCsumErrors:
		fallthrough
	case metricSndbufErrors:
		fallthrough
	case metricRcvbufErrors:
		fallthrough
	case metricNoPorts:
		fallthrough
	case metricInErrors:
		tagList = append(tagList, tags.Tag{Category: "units", Value: "errors"})
	default:
		c.logger.Warn().Str("protocol", proto).Str("metric", name).Msg("unrecognized metric, no units")
	}
	_ = c.addMetric(metrics, name, "L", val, tagList)
}
