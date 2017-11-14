// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package prom

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	cgm "github.com/circonus-labs/circonus-gometrics"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// New creates new prom collector
func New(cfgBaseName string) (collector.Collector, error) {
	c := Prom{
		metricStatus:        map[string]bool{},
		metricDefaultActive: true,
		include:             defaultIncludeRegex,
		exclude:             defaultExcludeRegex,
	}
	c.pkgID = "builtins.promfetc"
	c.logger = log.With().Str("pkg", c.pkgID).Logger()

	// Prom is a special builtin, it requires a configuration file,
	// it does not work if there are no prom urls to pull metrics from.
	// default config is a file named promfetch.(json|toml|yaml) in the
	// agent's default etc path.
	if cfgBaseName == "" {
		cfgBaseName = path.Join(defaults.EtcPath, "promfetch")
	}

	var opts promOptions
	err := config.LoadConfigFile(cfgBaseName, &opts)
	if err != nil {
		c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, errors.Wrapf(err, "%s config", c.pkgID)
	}

	c.logger.Debug().Str("base", cfgBaseName).Interface("config", opts).Msg("loaded config")

	if len(opts.URLs) == 0 {
		return nil, errors.New("'urls' is REQUIRED in configuration")
	}

	c.urls = append(c.urls, opts.URLs...)

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

// Collect returns collector metrics
func (c *Prom) Collect() error {
	metrics := cgm.Metrics{}
	c.Lock()

	if c.running {
		c.logger.Warn().Msg(collector.ErrAlreadyRunning.Error())
		c.Unlock()
		return collector.ErrAlreadyRunning
	}

	if c.runTTL > time.Duration(0) {
		if time.Since(c.lastEnd) < c.runTTL {
			c.logger.Warn().Msg(collector.ErrTTLNotExpired.Error())
			c.Unlock()
			return collector.ErrTTLNotExpired
		}
	}

	c.running = true
	c.lastStart = time.Now()
	c.Unlock()

	for _, u := range c.urls {
		c.logger.Debug().Str("id", u.ID).Str("url", u.URL).Msg("prom fetch request")
	}

	c.setStatus(metrics, nil)
	return nil
}
