// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package builtins marshals internal (non-plugin) metric collectors (e.g. procfs, wmi, etc.)
package builtins

import (
	"context"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector/prometheus"
	"github.com/circonus-labs/circonus-agent/internal/config"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	appstats "github.com/maier/go-appstats"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// Builtins defines the internal metric collector manager
type Builtins struct {
	collectors map[string]collector.Collector
	logger     zerolog.Logger
	running    bool
	sync.Mutex
}

// New creates a new builtins manager
func New(ctx context.Context) (*Builtins, error) {
	b := Builtins{
		collectors: make(map[string]collector.Collector),
		logger:     log.With().Str("pkg", "builtins").Logger(),
	}

	b.logger.Info().Msg("configuring builtins")

	if viper.GetBool(config.KeyClusterEnabled) && !viper.GetBool(config.KeyClusterEnableBuiltins) {
		b.logger.Info().Msg("cluster mode - builtins disabled")
		return &b, nil
	}

	err := b.configure(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "configuring builtins")
	}

	// prom applies to all platforms
	prom, err := prometheus.New("")
	if err != nil {
		b.logger.Warn().Err(err).Msg("prom collector, disabling")
	} else {
		// presence of a config is what enables this plugin
		// if no error, and no config both `prom` and `err` will be nil, so silently disable
		if prom != nil {
			b.logger.Info().Str("id", "prom").Msg("enabled builtin")
			b.collectors[prom.ID()] = prom
			_ = appstats.IncrementInt("builtins.total")
		}
	}

	return &b, nil
}

// Run triggers internal collectors to gather metrics
func (b *Builtins) Run(ctx context.Context, id string) error {
	b.Lock()

	if len(b.collectors) == 0 {
		b.Unlock()
		return nil // nothing to do
	}

	if b.running {
		b.logger.Warn().Msg("already in progress")
		b.Unlock()
		return nil
	}

	b.running = true
	b.Unlock()

	start := time.Now()
	if err := appstats.SetString("builtins.last_start", start.String()); err != nil {
		b.logger.Warn().Err(err).Msg("setting app stat")
	}

	var wg sync.WaitGroup

	if id == "" {
		wg.Add(len(b.collectors))
		for id, c := range b.collectors {
			clog := c.Logger()
			clog.Debug().Msg("collecting")
			go func(id string, c collector.Collector) {
				err := c.Collect(ctx)
				if err != nil {
					clog.Error().Err(err).Msg(id)
				}
				clog.Debug().Str("duration", time.Since(start).String()).Msg("done")
				wg.Done()
			}(id, c)
		}
	} else {
		c, ok := b.collectors[id]
		if ok {
			wg.Add(1)
			clog := c.Logger()
			clog.Debug().Msg("collecting")
			go func(id string, c collector.Collector) {
				err := c.Collect(ctx)
				if err != nil {
					clog.Error().Err(err).Msg(id)
				}
				clog.Debug().Str("duration", time.Since(start).String()).Msg("done")
				wg.Done()
			}(id, c)
		} else {
			b.logger.Warn().Str("id", id).Msg("unknown builtin")
		}
	}

	wg.Wait()

	b.logger.Debug().Msg("all builtins done")

	if err := appstats.SetString("builtins.last_end", time.Now().String()); err != nil {
		b.logger.Warn().Err(err).Msg("setting app stat")
	}
	if err := appstats.SetString("builtins.last_duration", time.Since(start).String()); err != nil {
		b.logger.Warn().Err(err).Msg("setting app stat")
	}

	b.Lock()
	b.running = false
	b.Unlock()

	return nil
}

// IsBuiltin determines if an id is a builtin or not
func (b *Builtins) IsBuiltin(id string) bool {
	if id == "" {
		return false
	}

	b.Lock()
	defer b.Unlock()

	if len(b.collectors) == 0 {
		return false
	}

	_, ok := b.collectors[id]

	return ok
}

// Flush returns current metrics for all collectors
func (b *Builtins) Flush(id string) *cgm.Metrics {
	b.Lock()
	defer b.Unlock()

	if err := appstats.SetString("builtins.last_flush", time.Now().String()); err != nil {
		b.logger.Warn().Err(err).Msg("setting app stat")
	}

	metrics := cgm.Metrics{}

	if len(b.collectors) == 0 {
		return &metrics // nothing to do
	}

	for _, c := range b.collectors {
		for name, val := range c.Flush() {
			metrics[name] = val
		}
	}

	return &metrics
}
