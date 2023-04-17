// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

//go:build linux
// +build linux

package procfs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
)

// NetProto metrics from the Linux ProcFS.
type NetProto struct {
	include *regexp.Regexp
	exclude *regexp.Regexp
	common
}

// netProtoOptions defines what elements can be overridden in a config file.
type netProtoOptions struct {
	// common
	ID         string `json:"id" toml:"id" yaml:"id"`
	ProcFSPath string `json:"procfs_path" toml:"procfs_path" yaml:"procfs_path"`
	RunTTL     string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`

	// collector specific
	IncludeRegex string `json:"include_regex" toml:"include_regex" yaml:"include_regex"`
	ExcludeRegex string `json:"exclude_regex" toml:"exclude_regex" yaml:"exclude_regex"`
}

// NewNetProtoCollector creates new procfs network protocol collector.
func NewNetProtoCollector(cfgBaseName, procFSPath string) (collector.Collector, error) {
	procFile := filepath.Join("net", "snmp")

	c := NetProto{
		common: newCommon(NameNetProto, procFSPath, procFile, tags.FromList(tags.GetBaseTags())),
	}

	c.include = defaultIncludeRegex
	c.exclude = defaultExcludeRegex

	if cfgBaseName == "" {
		if _, err := os.Stat(c.file); os.IsNotExist(err) {
			return nil, fmt.Errorf("%s procfile: %w", c.pkgID, err)
		}
		return &c, nil
	}

	var opts netProtoOptions
	err := config.LoadConfigFile(cfgBaseName, &opts)
	if err != nil {
		if !strings.Contains(err.Error(), "no config found matching") {
			c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
			return nil, fmt.Errorf("%s config: %w", c.pkgID, err)
		}
	} else {
		c.logger.Debug().Str("base", cfgBaseName).Interface("config", opts).Msg("loaded config")
	}

	if opts.IncludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, opts.IncludeRegex))
		if err != nil {
			return nil, fmt.Errorf("%s compile include rx: %w", c.pkgID, err)
		}
		c.include = rx
	}

	if opts.ExcludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, opts.ExcludeRegex))
		if err != nil {
			return nil, fmt.Errorf("%s compile exclude rx: %w", c.pkgID, err)
		}
		c.exclude = rx
	}

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
			return nil, fmt.Errorf("%s parsing run_ttl: %w", c.pkgID, err)
		}
		c.runTTL = dur
	}

	if _, err := os.Stat(c.file); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s read file: %w", c.pkgID, err)
	}

	return &c, nil
}

// Collect metrics from the procfs resource.
func (c *NetProto) Collect(ctx context.Context) error {
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

	if err := c.snmpCollect(ctx, &metrics); err != nil {
		c.setStatus(metrics, err)
		return fmt.Errorf("%s snmpCollect: %w", c.pkgID, err)
	}

	c.setStatus(metrics, nil)
	return nil
}

type rawSNMPStat struct {
	name string
	val  string
}

// snmpCollect gets metrics from /proc/net/snmp and /proc/net/snmp6.
func (c *NetProto) snmpCollect(ctx context.Context, metrics *cgm.Metrics) error {

	stats := make(map[string][]rawSNMPStat)

	{
		// snmp
		/*
			Ip: Forwarding DefaultTTL InReceives InHdrErrors InAddrErrors ForwDatagrams InUnknownProtos InDiscards InDelivers OutRequests OutDiscards OutNoRoutes ReasmTimeout ReasmReqds ReasmOKs ReasmFails FragOKs FragFails FragCreates
			Ip: 2 64 24480 0 0 0 0 0 24476 20850 15 0 0 0 0 0 0 0 0
			Icmp: InMsgs InErrors InCsumErrors InDestUnreachs InTimeExcds InParmProbs InSrcQuenchs InRedirects InEchos InEchoReps InTimestamps InTimestampReps InAddrMasks InAddrMaskReps OutMsgs OutErrors OutDestUnreachs OutTimeExcds OutParmProbs OutSrcQuenchs OutRedirects OutEchos OutEchoReps OutTimestamps OutTimestampReps OutAddrMasks OutAddrMaskReps
			Icmp: 32 0 0 32 0 0 0 0 0 0 0 0 0 0 33 0 33 0 0 0 0 0 0 0 0 0 0
			IcmpMsg: InType3 OutType3
			IcmpMsg: 32 33
			Tcp: RtoAlgorithm RtoMin RtoMax MaxConn ActiveOpens PassiveOpens AttemptFails EstabResets CurrEstab InSegs OutSegs RetransSegs InErrs OutRsts InCsumErrors
			Tcp: 1 200 120000 -1 94 3 0 0 1 24052 20416 0 0 21 0
			Udp: InDatagrams NoPorts InErrors OutDatagrams RcvbufErrors SndbufErrors InCsumErrors
			Udp: 359 33 0 404 0 0 0
			UdpLite: InDatagrams NoPorts InErrors OutDatagrams RcvbufErrors SndbufErrors InCsumErrors
			UdpLite: 0 0 0 0 0 0 0
		*/
		lines, err := c.readFile(c.file)
		if err != nil {
			return fmt.Errorf("%s read file: %w", c.pkgID, err)
		}
		for _, line := range lines {
			if done(ctx) {
				return fmt.Errorf("context: %w", ctx.Err())
			}

			fields := strings.Fields(line)

			proto := strings.ToLower(strings.ReplaceAll(fields[0], ":", ""))

			if c.exclude.MatchString(proto) || !c.include.MatchString(proto) {
				c.logger.Debug().Str("proto", proto).Msg("excluded, skipping")
				continue
			}

			if strings.ContainsAny(fields[1], "abcdefghijklmnopqrstuvwxyz") {
				// header row
				stats[proto] = make([]rawSNMPStat, len(fields))
				for i := 1; i < len(fields); i++ {
					stats[proto][i].name = fields[i]
				}
				continue
			}

			// stats row
			for i := 1; i < len(fields); i++ {
				stats[proto][i].val = fields[i]
			}
		}
	}
	{
		// snmp6
		snmp6File := c.file + "6"
		lines, err := c.readFile(snmp6File)
		if err != nil {
			return fmt.Errorf("%s read file: %w", c.pkgID, err)
		}
		for _, line := range lines {
			if done(ctx) {
				return fmt.Errorf("context: %w", ctx.Err())
			}

			fields := strings.Fields(line)
			if len(fields) != 2 {
				continue
			}
			protoFields := strings.Split(fields[0], "6")
			if len(protoFields) != 2 {
				continue
			}
			proto := strings.ToLower(protoFields[0]) + "6"
			statName := protoFields[1]

			if c.exclude.MatchString(proto) || !c.include.MatchString(proto) {
				c.logger.Debug().Str("proto", proto).Msg("excluded, skipping")
				continue
			}

			if _, ok := stats[proto]; !ok {
				stats[proto] = []rawSNMPStat{}
			}
			stats[proto] = append(stats[proto], rawSNMPStat{name: statName, val: fields[1]})
		}
	}

	for proto, protoStats := range stats {
		if done(ctx) {
			return fmt.Errorf("context: %w", ctx.Err())
		}

		for _, stat := range protoStats {
			if done(ctx) {
				return fmt.Errorf("context: %w", ctx.Err())
			}

			if stat.name == "" || stat.val == "" {
				continue
			}
			v, err := strconv.ParseInt(stat.val, 10, 64)
			if err != nil {
				c.logger.Warn().Err(err).Str("proto", proto).Str("name", stat.name).Msg("parsing field")
				continue
			}

			switch proto {
			case "icmp", "icmp6":
				c.emitICMPMetric(proto, metrics, stat.name, v)
			case "icmpmsg", "icmpmsg6":
				c.emitICMPMsgMetric(proto, metrics, stat.name, v)
			case "ip", "ip6":
				c.emitIPMetric(proto, metrics, stat.name, v)
			case "tcp", "tcp6":
				c.emitTCPMetric(proto, metrics, stat.name, v)
			case "udp", "udp6":
				c.emitUDPMetric(proto, metrics, stat.name, v)
			case "udplite", "udplite6":
				c.emitUDPLiteMetric(proto, metrics, stat.name, v)
			default:
				c.logger.Warn().Str("proto", proto).Msg("unsupported protocol")
			}
		}
	}

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
	metricIgnoredMulti = "IgnoredMulti"
	defaultMetricType  = "l"
)

func (c *NetProto) emitIPMetric(proto string, metrics *cgm.Metrics, name string, val int64) {

	// https://sourceforge.net/p/net-tools/code/ci/master/tree/statistics.c#l56
	// https://tools.ietf.org/html/rfc1213

	tagUnitsSeconds := tags.Tag{Category: "units", Value: "seconds"}
	tagUnitsDatagrams := tags.Tag{Category: "units", Value: "datagrams"}
	tagUnitsFragments := tags.Tag{Category: "units", Value: "fragments"}
	tagUnitsPackets := tags.Tag{Category: "units", Value: "packets"}
	tagUnitsOctets := tags.Tag{Category: "units", Value: "octets"}
	tagUnitsRequests := tags.Tag{Category: "units", Value: "requests"}

	tagList := tags.Tags{tags.Tag{Category: "proto", Value: proto}}

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

	// IPv6

	case "InTooBigErrors":
		// no units
	case "InNoRoutes":
		// no units
	case "InTruncatedPkts":
		tagList = append(tagList, tagUnitsPackets)
	case "InMcastPkts":
		tagList = append(tagList, tagUnitsPackets)
	case "InOctets":
		tagList = append(tagList, tagUnitsOctets)
	case "InMcastOctets":
		tagList = append(tagList, tagUnitsOctets)
	case "InBcastOctets":
		tagList = append(tagList, tagUnitsOctets)
	case "InNoECTPkts":
		tagList = append(tagList, tagUnitsPackets)
	case "InECT1Pkts":
		tagList = append(tagList, tagUnitsPackets)
	case "InECT0Pkts":
		tagList = append(tagList, tagUnitsPackets)
	case "InCEPkts":
		tagList = append(tagList, tagUnitsPackets)
	case "OutForwDatagrams":
		tagList = append(tagList, tagUnitsDatagrams)
	case "OutMcastPkts":
		tagList = append(tagList, tagUnitsPackets)
	case "OutOctets":
		tagList = append(tagList, tagUnitsOctets)
	case "OutMcastOctets":
		tagList = append(tagList, tagUnitsOctets)
	case "OutBcastOctets":
		tagList = append(tagList, tagUnitsOctets)
	default:
		c.logger.Warn().Str("protocol", proto).Str("metric", name).Msg("unrecognized metric, no units")
	}

	_ = c.addMetric(metrics, "", name, defaultMetricType, val, tagList)
}

func (c *NetProto) emitICMPMetric(proto string, metrics *cgm.Metrics, name string, val int64) {

	// https://sourceforge.net/p/net-tools/code/ci/master/tree/statistics.c#l105
	// https://tools.ietf.org/html/rfc1213

	tagUnitsResponses := tags.Tag{Category: "units", Value: "responses"}
	tagUnitsRequests := tags.Tag{Category: "units", Value: "requests"}
	tagUnitsMessages := tags.Tag{Category: "units", Value: "messages"}
	tagUnitsRedirects := tags.Tag{Category: "units", Value: "redirects"}
	tagUnitsPackets := tags.Tag{Category: "units", Value: "packets"}

	tagList := tags.Tags{tags.Tag{Category: "proto", Value: proto}}
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
	case "InType1":
		// no units

	//
	// ICMPv6 specific
	//
	case "InEchoReplies":
		tagList = append(tagList, tagUnitsResponses)
	case "InGroupMembQueries":
		// no units
	case "InGroupMembResponses":
		tagList = append(tagList, tagUnitsResponses)
	case "InGroupMembReductions":
		// no units
	case "InRouterSolicits":
		// no units
	case "InRouterAdvertisements":
		// no units
	case "InNeighborSolicits", "InType135":
		// no units
	case "InNeighborAdvertisements":
		// no units
	case "InPktTooBigs":
		tagList = append(tagList, tagUnitsPackets)
	case "InParmProblems":
		// no units
	case "InMLDv2Reports":
		// no units
	case "OutEchoReplies":
		tagList = append(tagList, tagUnitsResponses)
	case "OutGroupMembQueries":
		// no units
	case "OutGroupMembResponses":
		tagList = append(tagList, tagUnitsResponses)
	case "OutGroupMembReductions":
		// no units
	case "OutRouterSolicits":
		// no units
	case "OutRouterAdvertisements":
		// no units
	case "OutNeighborSolicits":
		// no units
	case "OutNeighborAdvertisements":
		// no units
	case "OutPktTooBigs":
		tagList = append(tagList, tagUnitsPackets)
	case "OutParmProblems":
		// no units
	case "OutMLDv2Reports":
		// no units
	case "OutType1":
		// no units
	case "OutType133":
		// no units
	case "OutType135":
		// no units
	case "OutType143":
		// no units
	case "OutType145":
		// no units
	default:
		c.logger.Warn().Str("protocol", proto).Str("metric", name).Msg("unrecognized metric, no units")
	}
	_ = c.addMetric(metrics, "", name, defaultMetricType, val, tagList)
}

func (c *NetProto) emitICMPMsgMetric(proto string, metrics *cgm.Metrics, name string, val int64) {

	// possible message types:
	// https://www.iana.org/assignments/icmp-parameters/icmp-parameters.xhtml

	tagList := tags.Tags{
		tags.Tag{Category: "proto", Value: proto},
		tags.Tag{Category: "units", Value: "messages"},
	}

	_ = c.addMetric(metrics, "", name, defaultMetricType, val, tagList)
}

func (c *NetProto) emitTCPMetric(proto string, metrics *cgm.Metrics, name string, val int64) {

	// https://sourceforge.net/p/net-tools/code/ci/master/tree/statistics.c#l170
	// https://tools.ietf.org/html/rfc1213

	tagUnitsConnections := tags.Tag{Category: "units", Value: "connections"}
	tagUnitsSegments := tags.Tag{Category: "units", Value: "segments"}
	tagUnitsResets := tags.Tag{Category: "units", Value: "resets"}
	tagUnitsMilliseconds := tags.Tag{Category: "units", Value: "milliseconds"}

	tagList := tags.Tags{tags.Tag{Category: "proto", Value: proto}}

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
	_ = c.addMetric(metrics, "", name, defaultMetricType, val, tagList)
}

func (c *NetProto) emitUDPMetric(proto string, metrics *cgm.Metrics, name string, val int64) {

	// https://sourceforge.net/p/net-tools/code/ci/master/tree/statistics.c#l188
	// https://tools.ietf.org/html/rfc1213

	tagUnitsDatagrams := tags.Tag{Category: "units", Value: "datagrams"}

	tagList := tags.Tags{tags.Tag{Category: "proto", Value: proto}}

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
	case metricIgnoredMulti:
		// no units
	default:
		c.logger.Warn().Str("protocol", proto).Str("metric", name).Msg("unrecognized metric, no units")
	}
	_ = c.addMetric(metrics, "", name, defaultMetricType, val, tagList)
}

func (c *NetProto) emitUDPLiteMetric(proto string, metrics *cgm.Metrics, name string, val int64) {

	// same names as UDP...

	tagUnitsDatagrams := tags.Tag{Category: "units", Value: "datagrams"}
	tagList := tags.Tags{tags.Tag{Category: "proto", Value: proto}}

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
	case metricIgnoredMulti:
		// no units
	default:
		c.logger.Warn().Str("protocol", proto).Str("metric", name).Msg("unrecognized metric, no units")
	}
	_ = c.addMetric(metrics, "", name, defaultMetricType, val, tagList)
}
