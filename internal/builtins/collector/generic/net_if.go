// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/net"
)

// IF metrics.
type IF struct {
	include *regexp.Regexp
	exclude *regexp.Regexp
	gencommon
}

// ifOptions defines what elements can be overridden in a config file.
type ifOptions struct {
	// common
	ID     string `json:"id" toml:"id" yaml:"id"`
	RunTTL string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`

	// collector specific
	IncludeRegex string `json:"include_regex" toml:"include_regex" yaml:"include_regex"`
	ExcludeRegex string `json:"exclude_regex" toml:"exclude_regex" yaml:"exclude_regex"`
}

// NewNetIFCollector creates new psutils collector.
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
		return nil, fmt.Errorf("%s config: %w", c.pkgID, err)
	}

	c.logger.Debug().Str("base", cfgBaseName).Interface("config", opts).Msg("loaded config")

	if opts.IncludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, opts.IncludeRegex))
		if err != nil {
			return nil, fmt.Errorf("%s compiling include regex: %w", c.pkgID, err)
		}
		c.include = rx
	}

	if opts.ExcludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, opts.ExcludeRegex))
		if err != nil {
			return nil, fmt.Errorf("%s compiling exclude regex: %w", c.pkgID, err)
		}
		c.exclude = rx
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
func (c *IF) Collect(ctx context.Context) error {
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

	metrics := cgm.Metrics{}
	ifaces, err := net.IOCounters(true)
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting network interface metrics")
		c.setStatus(metrics, nil)
		return nil
	}

	// units:packets
	tagUnitsPackets := tags.Tag{Category: "units", Value: "packets"}
	// units:bytes
	tagUnitsBytes := tags.Tag{Category: "units", Value: "bytes"}

	for _, iface := range ifaces {
		if c.exclude.MatchString(iface.Name) || !c.include.MatchString(iface.Name) {
			c.logger.Debug().Str("iface", iface.Name).Msg("excluded iface name, skipping")
			continue
		}

		// interface tag(s)
		ifTags := tags.Tags{tags.Tag{Category: "network-interface", Value: iface.Name}}

		{
			var tagList tags.Tags
			tagList = append(tagList, tagUnitsBytes)
			tagList = append(tagList, ifTags...)
			_ = c.addMetric(&metrics, "sent", "L", iface.BytesSent, tagList)
			_ = c.addMetric(&metrics, "recv", "L", iface.BytesRecv, tagList)
		}

		{
			var tagList tags.Tags
			tagList = append(tagList, tagUnitsPackets)
			tagList = append(tagList, ifTags...)
			_ = c.addMetric(&metrics, "sent", "L", iface.PacketsSent, tagList)
			_ = c.addMetric(&metrics, "recv", "L", iface.PacketsRecv, tagList)
		}

		{
			// directional in|out

			inTags := tags.Tags{tags.Tag{Category: "direction", Value: "in"}}
			inTags = append(inTags, ifTags...)

			outTags := tags.Tags{tags.Tag{Category: "direction", Value: "out"}}
			outTags = append(outTags, ifTags...)

			// fifo - no units
			_ = c.addMetric(&metrics, "fifo", "L", iface.Fifoin, inTags)
			_ = c.addMetric(&metrics, "fifo", "L", iface.Fifoout, outTags)

			// errors - no units
			_ = c.addMetric(&metrics, "errors", "L", iface.Errin, inTags)
			_ = c.addMetric(&metrics, "errors", "L", iface.Errout, outTags)

			// drops
			{
				var tagList tags.Tags
				tagList = append(tagList, inTags...)
				tagList = append(tagList, tagUnitsPackets)
				_ = c.addMetric(&metrics, "drops", "L", iface.Dropin, tagList)
			}
			{
				var tagList tags.Tags
				tagList = append(tagList, outTags...)
				tagList = append(tagList, tagUnitsPackets)
				_ = c.addMetric(&metrics, "drops", "L", iface.Dropout, tagList)
			}
		}
	}

	c.setStatus(metrics, nil)
	return nil
}
