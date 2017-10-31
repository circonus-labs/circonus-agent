// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package procfs

import (
	"path"
	"runtime"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// New creates new ProcFS collector
func New() ([]collector.Collector, error) {
	none := []collector.Collector{}

	if runtime.GOOS != "linux" {
		return none, nil
	}

	l := log.With().Str("pkg", "builtins.procfs").Logger()

	enbledCollectors := viper.GetStringSlice(config.KeyCollectors)
	if len(enbledCollectors) == 0 {
		l.Info().Msg("no builtin collectors enabled")
		return none, nil
	}

	collectors := make([]collector.Collector, len(enbledCollectors))
	for _, name := range enbledCollectors {
		switch name {
		case "cpu":
			c, err := NewCPUCollector(path.Join(defaults.EtcPath, "cpu"))
			if err != nil {
				l.Error().
					Str("name", name).
					Err(err).
					Msg("initializing builtin collector")
			} else {
				collectors = append(collectors, c)
			}
		default:
			l.Warn().
				Str("name", name).
				Msg("unknown builtin collector, ignoring")
		}
	}

	return collectors, nil
}
