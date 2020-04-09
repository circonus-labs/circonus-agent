// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build windows

package builtins

import (
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector/generic"
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector/windows/nvidia"
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector/windows/wmi"
	appstats "github.com/maier/go-appstats"
	"github.com/rs/zerolog/log"
)

func (b *Builtins) configure() error {
	l := log.With().Str("pkg", "builtins").Logger()

	{
		// WMI collectors
		l.Debug().Msg("calling wmi.New")
		collectors, err := wmi.New()
		if err != nil {
			return err
		}
		for _, c := range collectors {
			b.logger.Info().Str("id", c.ID()).Msg("enabled wmi builtin")
			b.collectors[c.ID()] = c
			_ = appstats.IncrementInt("builtins.total")
		}
	}

	{
		// Nvidia collector(s)
		l.Debug().Msg("calling nvidia.new")
		collectors, err := nvidia.New()
		if err != nil {
			return err
		}
		for _, c := range collectors {
			b.logger.Info().Str("id", c.ID()).Msg("enabled nvidia builtin")
			b.collectors[c.ID()] = c
			_ = appstats.IncrementInt("builtins.total")
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
				b.logger.Info().Str("id", c.ID()).Msg("enabled generic builtin")
				b.collectors[c.ID()] = c
				_ = appstats.IncrementInt("builtins.total")
			}
		}
	}

	return nil
}
