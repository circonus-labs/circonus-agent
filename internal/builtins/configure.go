// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build !windows,!linux

package builtins

import (
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector/generic"
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector/prometheus"
	appstats "github.com/maier/go-appstats"
	"github.com/rs/zerolog/log"
)

func (b *Builtins) configure() error {
	l := log.With().Str("pkg", "builtins").Logger()

	l.Debug().Msg("calling generic.New")
	collectors, err := generic.New()
	if err != nil {
		return err
	}
	for _, c := range collectors {
		appstats.IncrementInt("builtins.total")
		b.logger.Info().Str("id", c.ID()).Msg("enabled builtin")
		b.collectors[c.ID()] = c
	}

	prom, err := prometheus.New("")
	if err != nil {
		b.logger.Warn().Err(err).Msg("prom collector, disabling")
	} else {
		b.collectors[prom.ID()] = prom
		appstats.IncrementInt("builtins.total")
	}

	return nil
}
