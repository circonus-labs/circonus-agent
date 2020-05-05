// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build windows

// Package nvidia collects GPU metrics using nvidia-smi.exe
package nvidia

import (
	"path"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// common defines common elements for metrics collector
type common struct {
	id              string               // id of the collector (used as metric name prefix)
	pkgID           string               // package prefix used for logging and errors
	lastEnd         time.Time            // last collection end time
	lastError       string               // last collection error
	metrics         *cgm.CirconusMetrics // metrics instance
	lastMetrics     cgm.Metrics          // last metrics collected
	lastRunDuration time.Duration        // last collection duration
	lastStart       time.Time            // last collection start time
	logger          zerolog.Logger       // collector logging instance
	// metricNameChar  string               // OPT character(s) used as replacement for metricNameRegex, may be overridden in config
	// metricNameRegex *regexp.Regexp       // OPT regex for cleaning names, may be overridden in config
	running  bool // is collector currently running
	baseTags tags.Tags
	sync.Mutex
}

const (
	prefix  = "nvidia/"
	pkgName = "builtins.windows.nvidia"
)

// New creates new Nvidia GPU collector
func New() ([]collector.Collector, error) {
	none := []collector.Collector{}
	l := log.With().Str("pkg", pkgName).Logger()

	if runtime.GOOS != "windows" {
		l.Warn().Msg("not windows, skipping nvidia")
		return none, nil
	}

	enbledCollectors := viper.GetStringSlice(config.KeyCollectors)
	if len(enbledCollectors) == 0 {
		l.Info().Msg("no builtin collectors enabled")
		return none, nil
	}

	logError := func(name string, err error) {
		l.Error().
			Str("name", name).
			Err(err).
			Msg("initializing builtin collector")
	}

	collectors := make([]collector.Collector, 0, len(enbledCollectors))
	for _, name := range enbledCollectors {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		name = strings.Replace(name, prefix, "", -1)
		cfgBase := "nvidia_" + name + "_collector"
		switch name {
		case "gpu":
			c, err := NewGPUCollector(path.Join(defaults.EtcPath, cfgBase))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		default:
			l.Warn().
				Str("name", name).
				Msg("unknown builtin collector for this OS, ignoring")
		}
	}

	return collectors, nil
}
