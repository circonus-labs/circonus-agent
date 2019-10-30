// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build windows

package builtins

import (
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector/generic"
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector/prometheus"
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector/windows/wmi"
	appstats "github.com/maier/go-appstats"
	"github.com/rs/zerolog/log"
)

func (b *Builtins) configure() error {
	l := log.With().Str("pkg", "builtins").Logger()

	{
		// WMI collecctors
		l.Debug().Msg("calling wmi.New")
		collectors, err := wmi.New()
		if err != nil {
			return err
		}
		for _, c := range collectors {
			_ = appstats.IncrementInt("builtins.total")
			b.logger.Info().Str("id", c.ID()).Msg("enabled wmi builtin")
			b.collectors[c.ID()] = c
		}
	}

	{
		// PSUtils
		// NOTE: enable any explicit generic builtins - wmi will take precdence if
		//       there is a metric namespace collision.
		//       e.g. if wmi.cpu and generic.cpu are both enabled, wmi.cpu will
		//       take precedence and the generic.cpu instance will be dropped.
		l.Debug().Msg("calling generic.New")
		collectors, err := generic.New()
		if err != nil {
			return err
		}
		for _, c := range collectors {
			if _, exists := b.collectors[c.ID()]; !exists {
				_ = appstats.IncrementInt("builtins.total")
				b.logger.Info().Str("id", c.ID()).Msg("enabled generic builtin")
				b.collectors[c.ID()] = c
			}
		}
	}

	prom, err := prometheus.New("")
	if err != nil {
		b.logger.Warn().Err(err).Msg("prom collector, disabling")
	} else {
		_ = appstats.IncrementInt("builtins.total")
		b.collectors[prom.ID()] = prom
	}
	return nil
}
