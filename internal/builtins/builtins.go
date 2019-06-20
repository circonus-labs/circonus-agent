// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package builtins marshals internal (non-plugin) metric collectors (e.g. procfs, wmi, etc.)
package builtins

import (
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	appstats "github.com/maier/go-appstats"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Builtins defines the internal metric collector manager
type Builtins struct {
	collectors map[string]collector.Collector
	logger     zerolog.Logger
	running    bool
	sync.Mutex
}

// New creates a new builtins manager
func New() (*Builtins, error) {
	b := Builtins{
		collectors: make(map[string]collector.Collector),
		logger:     log.With().Str("pkg", "builtins").Logger(),
	}

	b.logger.Info().Msg("configuring builtins")

	err := b.configure()
	if err != nil {
		return nil, errors.Wrap(err, "configuring builtins")
	}

	return &b, nil
}

// Run triggers internal collectors to gather metrics
func (b *Builtins) Run(id string) error {
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
	appstats.SetString("builtins.last_start", start.String())

	var wg sync.WaitGroup

	if id == "" {
		wg.Add(len(b.collectors))
		for id, c := range b.collectors {
			clog := c.Logger()
			clog.Debug().Msg("collecting")
			go func(id string, c collector.Collector) {
				err := c.Collect()
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
				err := c.Collect()
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

	appstats.SetString("builtins.last_end", time.Now().String())
	appstats.SetString("builtins.last_duration", time.Since(start).String())

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

	appstats.SetString("builtins.last_flush", time.Now().String())

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
