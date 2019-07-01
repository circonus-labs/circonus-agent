// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build linux

package procfs

import (
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
)

// NetIF metrics from the Linux ProcFS
type NetIF struct {
	common
	include *regexp.Regexp
	exclude *regexp.Regexp
}

// netIFOptions defines what elements can be overridden in a config file
type netIFOptions struct {
	// common
	ID         string `json:"id" toml:"id" yaml:"id"`
	ProcFSPath string `json:"procfs_path" toml:"procfs_path" yaml:"procfs_path"`
	RunTTL     string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`

	// collector specific
	IncludeRegex string `json:"include_regex" toml:"include_regex" yaml:"include_regex"`
	ExcludeRegex string `json:"exclude_regex" toml:"exclude_regex" yaml:"exclude_regex"`
}

// NewNetIFCollector creates new procfs if collector
func NewNetIFCollector(cfgBaseName, procFSPath string) (collector.Collector, error) {
	procFile := filepath.Join("net", "dev")

	c := NetIF{
		common: newCommon(NameNetInterface, procFSPath, procFile, tags.FromList(tags.GetBaseTags())),
	}

	c.include = defaultIncludeRegex
	c.exclude = regexp.MustCompile(fmt.Sprintf(regexPat, `lo`))

	if cfgBaseName == "" {
		if _, err := os.Stat(c.file); os.IsNotExist(err) {
			return nil, errors.Wrap(err, c.pkgID)
		}
		return &c, nil
	}

	var opts netIFOptions
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
func (c *NetIF) Collect() error {
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
		c.setStatus(metrics, err)
		return errors.Wrap(err, c.pkgID)
	}

	c.setStatus(metrics, nil)
	return nil
}

// ifCollect gets metrics from /proc/net/dev
func (c *NetIF) ifCollect(metrics *cgm.Metrics) error {
	unitBytesTag := tags.Tag{Category: "units", Value: "bytes"}
	unitPacketsTag := tags.Tag{Category: "units", Value: "packets"}
	dirInTag := tags.Tag{Category: "direction", Value: "in"}
	dirOutTag := tags.Tag{Category: "direction", Value: "out"}

	// https://github.com/torvalds/linux/blob/master/net/core/net-procfs.c#L78
	//  1 interface name
	//  2 receive bytes
	//  3 receive packets
	//  4 receive errs
	//  5 receive drop
	//  6 receive fifo
	//  7 receive frame // length, over, crc, frame errors
	//  8 receive compressed
	//  9 receive multicast
	// 10 transmit bytes
	// 11 transmit packets
	// 12 transmit errs
	// 13 transmit drop
	// 14 transmit fifo
	// 15 transmit colls
	// 16 transmit carrier // carrier, aborted, window, and heartbeat errors
	// 17 transmit compressed
	fieldsExpected := 17
	stats := []struct {
		idx   int
		name  string
		desc  string
		stags tags.Tags
	}{
		{idx: 1, name: "recv", desc: "receive bytes", stags: tags.Tags{unitBytesTag}},
		{idx: 2, name: "recv", desc: "receive packets", stags: tags.Tags{unitPacketsTag}},
		{idx: 3, name: "errors", desc: "receive errors", stags: tags.Tags{dirInTag}},
		{idx: 4, name: "drops", desc: "receive dropped + missed errors", stags: tags.Tags{dirInTag, unitPacketsTag}},
		{idx: 5, name: "fifo", desc: "receive fifo errors", stags: tags.Tags{dirInTag}},
		{idx: 6, name: "frame", desc: "recevie (length,over,crc,frame) errors", stags: tags.Tags{dirInTag}},
		{idx: 7, name: "compressed", desc: "receive compressed", stags: tags.Tags{dirInTag}},
		{idx: 8, name: "multicast", desc: "receive multicast", stags: tags.Tags{unitPacketsTag}},
		{idx: 9, name: "sent", desc: "transmit bytes", stags: tags.Tags{unitBytesTag}},
		{idx: 10, name: "sent", desc: "transmit packets", stags: tags.Tags{unitPacketsTag}},
		{idx: 11, name: "errors", desc: "transmit errors", stags: tags.Tags{dirOutTag}},
		{idx: 12, name: "drops", desc: "transmit drop", stags: tags.Tags{dirOutTag, unitPacketsTag}},
		{idx: 13, name: "fifo", desc: "trasnmit fifo errors", stags: tags.Tags{dirOutTag}},
		{idx: 14, name: "collision", desc: "transmit collisions", stags: tags.Tags{dirOutTag}},
		{idx: 15, name: "carrier", desc: "transmit (aborted, window, heartbeat, carrier) errors", stags: tags.Tags{dirOutTag}},
		{idx: 16, name: "compressed", desc: "transmit compressed", stags: tags.Tags{dirOutTag}},
	}

	metricType := "L" // uint64

	lines, err := c.readFile(c.file)
	if err != nil {
		return errors.Wrapf(err, "parsing %s", c.file)
	}
	for _, line := range lines {
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
				c.logger.Warn().Err(err).Str("iface", iface).Int("idx", s.idx).Str("desc", s.desc).Msg("missing field")
				continue
			}

			v, err := strconv.ParseUint(fields[s.idx], 10, 64)
			if err != nil {
				c.logger.Warn().Err(err).Str("iface", iface).Int("idx", s.idx).Str("desc", s.desc).Msg("parsing field")
				continue
			}

			tagList := tags.Tags{tags.Tag{Category: "network-interface", Value: iface}}
			tagList = append(tagList, s.stags...)
			_ = c.addMetric(metrics, "", s.name, metricType, v, tagList)
		}
	}

	return nil
}
