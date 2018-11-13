// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package psuc

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

const (
	PSUC_PREFIX         = "psuc/"
	LOG_PREFIX          = "builtins.psuc."
	CPU_NAME            = "cpu"
	DISK_NAME           = "disk"
	metricNameSeparator = "`"        // character used to separate parts of metric names
	metricStatusEnabled = "enabled"  // setting string indicating metrics should be made 'active'
	regexPat            = `^(?:%s)$` // fmt pattern used compile include/exclude regular expressions
)

var (
	defaultExcludeRegex = regexp.MustCompile(fmt.Sprintf(regexPat, ""))
	defaultIncludeRegex = regexp.MustCompile(fmt.Sprintf(regexPat, ".+"))
)

// New creates new PSUtil collector
func New() ([]collector.Collector, error) {
	none := []collector.Collector{}

	l := log.With().Str("pkg", "builtins.psuc").Logger()

	enbledCollectors := viper.GetStringSlice(config.KeyCollectors)
	if len(enbledCollectors) == 0 {
		l.Info().Msg("no builtin collectors enabled")
		return none, nil
	}

	collectors := make([]collector.Collector, 0, len(enbledCollectors))
	initErrMsg := "initializing builtin collector"
	for _, name := range enbledCollectors {
		if !strings.HasPrefix(name, PSUC_PREFIX) {
			continue
		}
		name = strings.Replace(name, PSUC_PREFIX, "", -1)
		cfgBase := "psuc_" + name + "_collector"
		switch name {
		case CPU_NAME:
			c, err := NewCPUCollector(path.Join(defaults.EtcPath, cfgBase))
			if err != nil {
				l.Error().Str("name", name).Err(err).Msg(initErrMsg)
				continue
			}
			collectors = append(collectors, c)

		case DISK_NAME:
			c, err := NewDiskCollector(path.Join(defaults.EtcPath, cfgBase))
			if err != nil {
				l.Error().Str("name", name).Err(err).Msg(initErrMsg)
				continue
			}
			collectors = append(collectors, c)

		default:
			l.Warn().Str("name", name).Msg("unknown builtin collector, ignoring")
		}
	}

	return collectors, nil
}
