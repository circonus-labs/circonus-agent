// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
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
		tags.Tag{Category: "module", Value: c.id},
	}

	metrics := cgm.Metrics{}
	counters, err := net.ProtoCounters(c.protocols)
	if err != nil {
		c.logger.Warn().Err(err).Msg("collecting network protocol metrics")
	} else {
		if len(counters) == 0 {
			return errors.New("no network protocol metrics available")
		}
		for _, counter := range counters {
			var tagList tags.Tags
			tagList = append(tagList, moduleTags...)
			tagList = append(tagList, tags.Tag{Category: "protocol", Value: counter.Protocol})
			for name, val := range counter.Stats {
				_ = c.addMetric(&metrics, name, "L", val, tagList)
			}
		}
	}

	c.setStatus(metrics, nil)
	return nil
}
