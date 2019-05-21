// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/net"
)

// IF metrics
type IF struct {
	gencommon
	include *regexp.Regexp
	exclude *regexp.Regexp
}

// ifOptions defines what elements can be overridden in a config file
type ifOptions struct {
	// common
	ID     string `json:"id" toml:"id" yaml:"id"`
	RunTTL string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`

	// collector specific
	IncludeRegex string `json:"include_regex" toml:"include_regex" yaml:"include_regex"`
	ExcludeRegex string `json:"exclude_regex" toml:"exclude_regex" yaml:"exclude_regex"`
}

// NewNetIFCollector creates new psutils collector
func NewNetIFCollector(cfgBaseName string, parentLogger zerolog.Logger) (collector.Collector, error) {
	c := IF{}
	c.id = NameIF
	c.pkgID = PackageName + "." + c.id
	c.logger = parentLogger.With().Str("id", c.id).Logger()
	c.baseTags = tags.FromList(tags.GetBaseTags())

	c.include = defaultIncludeRegex
	c.exclude = regexp.MustCompile(fmt.Sprintf(regexPat, `lo`))

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
func (c *IF) Collect() error {
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
		tags.Tag{Category: "module", Value: c.id},
	}

	metrics := cgm.Metrics{}
	ifaces, err := net.IOCounters(true)
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting network interface metrics")
	} else {
		for _, iface := range ifaces {
			if c.exclude.MatchString(iface.Name) || !c.include.MatchString(iface.Name) {
				c.logger.Debug().Str("iface", iface.Name).Msg("excluded iface name, skipping")
				continue
			}
			var tagList tags.Tags
			tagList = append(tagList, moduleTags...)
			tagList = append(tagList, tags.Tag{Category: "interface", Value: iface.Name})

			_ = c.addMetric(&metrics, "sent_bytes", "L", iface.BytesSent, tagList)
			_ = c.addMetric(&metrics, "recv_bytes", "L", iface.BytesRecv, tagList)
			_ = c.addMetric(&metrics, "sent_pkts", "L", iface.PacketsSent, tagList)
			_ = c.addMetric(&metrics, "recv_pkts", "L", iface.PacketsRecv, tagList)
			_ = c.addMetric(&metrics, "in_errors", "L", iface.Errin, tagList)
			_ = c.addMetric(&metrics, "out_errors", "L", iface.Errout, tagList)
			_ = c.addMetric(&metrics, "in_drops", "L", iface.Dropin, tagList)
			_ = c.addMetric(&metrics, "out_drops", "L", iface.Dropout, tagList)
			_ = c.addMetric(&metrics, "in_fifo", "L", iface.Fifoin, tagList)
			_ = c.addMetric(&metrics, "out_fifo", "L", iface.Fifoout, tagList)
		}
	}

	c.setStatus(metrics, nil)
	return nil
}
