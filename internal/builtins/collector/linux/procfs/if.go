// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build linux

package procfs

import (
	"bufio"
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
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// IF metrics from the Linux ProcFS
type IF struct {
	pfscommon
	include *regexp.Regexp
	exclude *regexp.Regexp
}

// ifOptions defines what elements can be overridden in a config file
type ifOptions struct {
	// common
	ID                   string   `json:"id" toml:"id" yaml:"id"`
	ProcFSPath           string   `json:"procfs_path" toml:"procfs_path" yaml:"procfs_path"`
	MetricsEnabled       []string `json:"metrics_enabled" toml:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsDisabled      []string `json:"metrics_disabled" toml:"metrics_disabled" yaml:"metrics_disabled"`
	MetricsDefaultStatus string   `json:"metrics_default_status" toml:"metrics_default_status" toml:"metrics_default_status"`
	RunTTL               string   `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`

	// collector specific
	IncludeRegex string `json:"include_regex" toml:"include_regex" yaml:"include_regex"`
	ExcludeRegex string `json:"exclude_regex" toml:"exclude_regex" yaml:"exclude_regex"`
}

// NewIFCollector creates new procfs if collector
func NewIFCollector(cfgBaseName, procFSPath string) (collector.Collector, error) {
	procFile := filepath.Join("net", "dev")

	c := IF{}
	c.id = IF_NAME
	c.pkgID = PFS_PREFIX + c.id
	c.procFSPath = procFSPath
	c.file = filepath.Join(c.procFSPath, procFile)
	c.logger = log.With().Str("pkg", c.pkgID).Logger()
	c.metricStatus = map[string]bool{}
	c.metricDefaultActive = true
	c.baseTags = tags.FromList(tags.GetBaseTags())

	c.include = defaultIncludeRegex
	c.exclude = regexp.MustCompile(fmt.Sprintf(regexPat, `lo`))

	if cfgBaseName == "" {
		if _, err := os.Stat(c.file); os.IsNotExist(err) {
			return nil, errors.Wrap(err, c.pkgID)
		}
		return &c, nil
	}

	var opts ifOptions
	err := config.LoadConfigFile(cfgBaseName, &opts)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, errors.Wrapf(err, "%s config", c.pkgID)
	}

	c.logger.Debug().Str("base", cfgBaseName).Interface("config", opts).Msg("loaded config")

	if opts.IncludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, opts.IncludeRegex))
		if err != nil {
			return nil, errors.Wrapf(err, "%s compiling include regex", c.pkgID)
		}
		c.include = rx
	}

	if opts.ExcludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, opts.ExcludeRegex))
		if err != nil {
			return nil, errors.Wrapf(err, "%s compiling exclude regex", c.pkgID)
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

	if len(opts.MetricsEnabled) > 0 {
		for _, name := range opts.MetricsEnabled {
			c.metricStatus[name] = true
		}
	}
	if len(opts.MetricsDisabled) > 0 {
		for _, name := range opts.MetricsDisabled {
			c.metricStatus[name] = false
		}
	}

	if opts.MetricsDefaultStatus != "" {
		if ok, _ := regexp.MatchString(`^(enabled|disabled)$`, strings.ToLower(opts.MetricsDefaultStatus)); ok {
			c.metricDefaultActive = strings.ToLower(opts.MetricsDefaultStatus) == metricStatusEnabled
		} else {
			return nil, errors.Errorf("%s invalid metric default status (%s)", c.pkgID, opts.MetricsDefaultStatus)
		}
	}

	if opts.RunTTL != "" {
		dur, err := time.ParseDuration(opts.RunTTL)
		if err != nil {
			return nil, errors.Wrapf(err, "%s parsing run_ttl", c.pkgID)
		}
		c.runTTL = dur
	}

	if _, err := os.Stat(c.file); os.IsNotExist(err) {
		return nil, errors.Wrap(err, c.pkgID)
	}

	return &c, nil
}

// Collect metrics from the procfs resource
func (c *IF) Collect() error {
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

	if err := c.ifCollect(&metrics); err != nil {
		c.setStatus(cgm.Metrics{}, err)
		return errors.Wrap(err, c.pkgID)
	}

	if err := c.snmpCollect(&metrics); err != nil {
		c.logger.Warn().Err(err).Msg("snmp")
	}

	if err := c.sockstatCollect(&metrics); err != nil {
		c.logger.Warn().Err(err).Msg("sockstat")
	}

	c.setStatus(metrics, nil)
	return nil
}

// ifCollect gets metrics from /proc/net/dev
func (c *IF) ifCollect(metrics *cgm.Metrics) error {
	f, err := os.Open(c.file)
	if err != nil {
		return errors.Wrap(err, "ifCollect")
	}
	defer f.Close()

	//  1 interface name
	//  2 receive bytes
	//  3 receive packets
	//  4 receive errs
	//  5 receive drop
	//  6 receive fifo
	//  7 receive frame
	//  8 receive compressed
	//  9 receive multicast
	// 10 transmit bytres
	// 11 transmit packets
	// 12 transmit errs
	// 13 transmit drop
	// 14 transmit fifo
	// 15 transmit colls
	// 16 transmit carrier
	// 17 transmit compressed
	fieldsExpected := 17
	stats := []struct {
		idx  int
		name string
		desc string
	}{
		{idx: 1, name: "in_bytes", desc: "receive bytes"},
		{idx: 2, name: "in_packets", desc: "receive packets"},
		{idx: 3, name: "in_errors", desc: "receive errs"},
		{idx: 4, name: "in_drop", desc: "receive drop"},
		{idx: 5, name: "in_fifo_overrun", desc: "receive fifo"},
		// {idx: 6, name: "in_frames", desc: "recevie frames"},
		// {idx: 7, name: "in_compressed", desc: "receive compressed"},
		// {idx: 8, name: "in_multicast", desc: "receive multicast"},
		{idx: 9, name: "out_bytes", desc: "transmit bytes"},
		{idx: 10, name: "out_packets", desc: "transmit packets"},
		{idx: 11, name: "out_errors", desc: "transmit errors"},
		{idx: 12, name: "out_drop", desc: "transmit drop"},
		{idx: 13, name: "out_fifo_overrun", desc: "trasnmit fifo"},
		// {idx: 14, name: "out_colls", desc: "transmit colls"},
		// {idx: 15, name: "out_carrier", desc: "transmit carrier"},
		// {idx: 16, name: "out_compressed", desc: "transmit compressed"},
	}

	scanner := bufio.NewScanner(f)
	pfx := c.id + metricNameSeparator
	metricType := "L" // uint64
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.Contains(line, "|") {
			continue // skip header lines
		}

		fields := strings.Fields(line)
		iface := strings.Replace(fields[0], ":", "", -1)

		if c.exclude.MatchString(iface) || !c.include.MatchString(iface) {
			c.logger.Debug().Str("iface", iface).Msg("excluded iface name, skipping")
			continue
		}

		if len(fields) != fieldsExpected {
			c.logger.Warn().Err(err).Str("iface", iface).Int("expected", fieldsExpected).Int("found", len(fields)).Msg("invalid number of fields")
			continue
		}

		for _, s := range stats {
			if len(fields) < s.idx {
				c.logger.Warn().Err(err).Str("iface", iface).Int("idx", s.idx).Msg("missing field " + s.name)
				continue
			}
			v, err := strconv.ParseUint(fields[s.idx], 10, 64)
			if err != nil {
				c.logger.Warn().Err(err).Str("iface", iface).Msg("parsing field " + s.desc)
				continue
			}
			c.addMetric(metrics, pfx+iface, s.name, metricType, v)
		}
	}

	if err := scanner.Err(); err != nil {
		return errors.Wrapf(err, "ifCollect parsing %s", f.Name())
	}

	return nil
}

type rawstat struct {
	name string
	val  string
}

// snmpCollect gets metrics from /proc/net/snmp
func (c *IF) snmpCollect(metrics *cgm.Metrics) error {
	snmpFile := strings.Replace(c.file, "dev", "snmp", -1)
	f, err := os.Open(snmpFile)
	if err != nil {
		return errors.Wrap(err, "snmpCollect")
	}
	defer f.Close()

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

	stats := make(map[string][]rawstat)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)

		statType := strings.ToLower(strings.Replace(fields[0], ":", "", -1))

		if strings.ContainsAny(fields[1], "abcdefghijklmnopqrstuvwxyz") {
			// header row
			stats[statType] = make([]rawstat, len(fields))
			for i := 1; i < len(fields); i++ {
				stats[statType][i].name = fields[i]
			}
		} else {
			// stats row
			for i := 1; i < len(fields); i++ {
				stats[statType][i].val = fields[i]
			}
		}
	}

	pfx := c.id + metricNameSeparator + "tcp"
	metricType := "L" // uint64
	for _, n := range stats["tcp"] {
		if n.name == "RetransSegs" {
			v, err := strconv.ParseUint(n.val, 10, 64)
			if err != nil {
				c.logger.Warn().Err(err).Msg("parsing tcp field " + n.name)
				break
			}
			c.addMetric(metrics, pfx, "segments_retransmitted", metricType, v)
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return errors.Wrapf(err, "snmpCollect parsing %s", f.Name())
	}

	return nil
}

// sockstatCollect gets metrics from /proc/net/sockstat and /proc/net/sockstat6
func (c *IF) sockstatCollect(metrics *cgm.Metrics) error {

	conns := uint64(0)

	{
		emsg := "sockstat - invalid number of fields"
		sockstatFile := strings.Replace(c.file, "dev", "sockstat", -1)
		f, err := os.Open(sockstatFile)
		if err != nil {
			return errors.Wrap(err, "sockstatCollect")
		}
		defer f.Close()

		/*
			sockets: used 176
			TCP: inuse 3 orphan 0 tw 0 alloc 5 mem 1
			UDP: inuse 3 mem 2
			UDPLITE: inuse 0
			RAW: inuse 0
			FRAG: inuse 0 memory 0
		*/

		stats := make(map[string][]rawstat)

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			fields := strings.Fields(line)

			statType := strings.ToLower(strings.Replace(fields[0], ":", "", -1))

			switch statType {
			case "sockets":
				if len(fields) != 3 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				stats[statType] = []rawstat{
					{
						name: fields[1], // used
						val:  fields[2],
					},
				}

			case "tcp":
				if len(fields) != 11 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				stats[statType] = []rawstat{
					{
						name: fields[1], // inuse
						val:  fields[2],
					},
					{
						name: fields[3], // orphan
						val:  fields[4],
					},
					{
						name: fields[5], // tw
						val:  fields[6],
					},
					{
						name: fields[7], // alloc
						val:  fields[8],
					},
					{
						name: fields[9], // mem
						val:  fields[10],
					},
				}

			case "udp":
				if len(fields) != 5 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				stats[statType] = []rawstat{
					{
						name: fields[1], // inuse
						val:  fields[2],
					},
					{
						name: fields[3], // mem
						val:  fields[4],
					},
				}

			case "udplite":
				if len(fields) != 3 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				stats[statType] = []rawstat{
					{
						name: fields[1], // inuse
						val:  fields[2],
					},
				}

			case "raw":
				if len(fields) != 3 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				stats[statType] = []rawstat{
					{
						name: fields[1], // inuse
						val:  fields[2],
					},
				}

			case "frag":
				if len(fields) != 5 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				stats[statType] = []rawstat{
					{
						name: fields[1], // inuse
						val:  fields[2],
					},
					{
						name: fields[3], // mem
						val:  fields[4],
					},
				}

			default:
				c.logger.Warn().Str("type", statType).Msg("sockstat - unknown stat type, ignoring")

			}
		}

		for _, n := range stats["tcp"] {
			if n.name == "inuse" {
				v, err := strconv.ParseUint(n.val, 10, 64)
				if err != nil {
					c.logger.Warn().Err(err).Msg("sockstat - parsing tcp field " + n.name)
					break
				}
				conns += v
				break
			}
		}

		if err := scanner.Err(); err != nil {
			return errors.Wrapf(err, "sockstatCollect parsing %s", f.Name())
		}
	}

	{
		emsg := "sockstat6 - invalid number of fields"
		sockstatFile := strings.Replace(c.file, "dev", "sockstat6", -1)
		f, err := os.Open(sockstatFile)
		if err != nil {
			return errors.Wrap(err, "sockstat6Collect")
		}
		defer f.Close()

		/*
		   TCP6: inuse 2
		   UDP6: inuse 2
		   UDPLITE6: inuse 0
		   RAW6: inuse 1
		   FRAG6: inuse 0 memory 0
		*/

		stats := make(map[string][]rawstat)

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			fields := strings.Fields(line)

			statType := strings.ToLower(strings.Replace(fields[0], ":", "", -1))

			switch statType {
			case "tcp6":
				if len(fields) != 3 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				stats[statType] = []rawstat{
					{
						name: fields[1], // inuse
						val:  fields[2],
					},
				}

			case "udp6":
				if len(fields) != 3 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				stats[statType] = []rawstat{
					{
						name: fields[1], // inuse
						val:  fields[2],
					},
				}

			case "udplite6":
				if len(fields) != 3 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				stats[statType] = []rawstat{
					{
						name: fields[1], // inuse
						val:  fields[2],
					},
				}

			case "raw6":
				if len(fields) != 3 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				stats[statType] = []rawstat{
					{
						name: fields[1], // inuse
						val:  fields[2],
					},
				}

			case "frag6":
				if len(fields) != 5 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				stats[statType] = []rawstat{
					{
						name: fields[1], // inuse
						val:  fields[2],
					},
					{
						name: fields[3], // memory
						val:  fields[4],
					},
				}

			default:
				c.logger.Warn().Str("type", statType).Msg("sockstat6 - unknown stat type, ignoring")

			}
		}

		for _, n := range stats["tcp6"] {
			if n.name == "inuse" {
				v, err := strconv.ParseUint(n.val, 10, 64)
				if err != nil {
					c.logger.Warn().Err(err).Msg("sockstat6 - parsing tcp6 field " + n.name)
					break
				}
				conns += v
				break
			}
		}

		if err := scanner.Err(); err != nil {
			return errors.Wrapf(err, "sockstatCollect parsing %s", f.Name())
		}
	}

	pfx := c.id + metricNameSeparator + "tcp"
	metricType := "L" // uint64
	c.addMetric(metrics, pfx, "connections", metricType, conns)

	return nil
}
