// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build linux

package builtins

import (
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector/generic"
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector/linux/procfs"
	appstats "github.com/maier/go-appstats"
	"github.com/rs/zerolog/log"
)

func (b *Builtins) configure() error {
	l := log.With().Str("pkg", "builtins").Logger()

	{
		// ProcFS
		// NOTE: these take precedence because the metrics emitted by these
		//       builtins are used by cosi visuals. these are _direct_ replacements
		//       for the original NAD plugins of the same name
		l.Debug().Msg("calling procfs.New")
		collectors, err := procfs.New()
		if err != nil {
			return err
		}
		for _, c := range collectors {
			b.logger.Info().Str("id", c.ID()).Msg("enabled procfs builtin")
			b.collectors[c.ID()] = c
			_ = appstats.IncrementInt("builtins.total")
		}
	}

	{
		// PSUtils
		// NOTE: psutils does not use the same metric names nor does it expose
		//       all of the same metrics as the original NAD plugins so it cannot
		//       be used as a replacement - any duplicates created will be ignored
		//       e.g. if procfs.cpu and generic.cpu are both enabled, procfs.cpu will
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
