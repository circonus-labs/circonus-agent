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
	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/net"
)

// IF metrics
type IF struct {
	common
	include *regexp.Regexp
	exclude *regexp.Regexp
}

// ifOptions defines what elements can be overridden in a config file
type ifOptions struct {
	// common
	ID                   string   `json:"id" toml:"id" yaml:"id"`
	MetricsEnabled       []string `json:"metrics_enabled" toml:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsDisabled      []string `json:"metrics_disabled" toml:"metrics_disabled" yaml:"metrics_disabled"`
	MetricsDefaultStatus string   `json:"metrics_default_status" toml:"metrics_default_status" toml:"metrics_default_status"`
	RunTTL               string   `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`

	// collector specific
	IncludeRegex string `json:"include_regex" toml:"include_regex" yaml:"include_regex"`
	ExcludeRegex string `json:"exclude_regex" toml:"exclude_regex" yaml:"exclude_regex"`
}

// NewNetIFCollector creates new psutils collector
func NewNetIFCollector(cfgBaseName string) (collector.Collector, error) {
	c := IF{}
	c.id = IF_NAME
	c.pkgID = LOG_PREFIX + c.id
	c.logger = log.With().Str("pkg", c.pkgID).Logger()
	c.metricStatus = map[string]bool{}
	c.metricDefaultActive = true
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
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", iface.Name, metricNameSeparator, "sent_bytes"), "L", iface.BytesSent)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", iface.Name, metricNameSeparator, "recv_bytes"), "L", iface.BytesRecv)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", iface.Name, metricNameSeparator, "sent_pkts"), "L", iface.PacketsSent)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", iface.Name, metricNameSeparator, "recv_pkts"), "L", iface.PacketsRecv)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", iface.Name, metricNameSeparator, "in_errors"), "L", iface.Errin)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", iface.Name, metricNameSeparator, "out_errors"), "L", iface.Errout)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", iface.Name, metricNameSeparator, "in_drops"), "L", iface.Dropin)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", iface.Name, metricNameSeparator, "out_drops"), "L", iface.Dropout)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", iface.Name, metricNameSeparator, "in_fifo"), "L", iface.Fifoin)
			c.addMetric(&metrics, c.id, fmt.Sprintf("%s%s%s", iface.Name, metricNameSeparator, "out_fifo"), "L", iface.Fifoout)
		}
	}

	c.setStatus(metrics, nil)
	return nil
}
