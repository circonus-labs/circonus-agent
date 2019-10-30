// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

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
	pkgPrefix           = "generic/"
	pkgName             = "builtins.generic"
	cpuName             = "cpu"
	diskName            = "disk"
	fsName              = "fs"
	loadName            = "load"
	vmName              = "vm"
	ifName              = "if"
	protoName           = "proto"
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

	l := log.With().Str("pkg", "builtins.generic").Logger()

	enbledCollectors := viper.GetStringSlice(config.KeyCollectors)
	if len(enbledCollectors) == 0 {
		l.Info().Msg("no builtin collectors enabled")
		return none, nil
	}

	collectors := make([]collector.Collector, 0, len(enbledCollectors))
	initErrMsg := "initializing builtin collector"
	for _, name := range enbledCollectors {
		if !strings.HasPrefix(name, pkgPrefix) {
			continue
		}
		name = strings.Replace(name, pkgPrefix, "", -1)
		cfgBase := "generic_" + name + "_collector"
		switch name {
		case cpuName:
			c, err := NewCPUCollector(path.Join(defaults.EtcPath, cfgBase))
			if err != nil {
				l.Error().Str("name", name).Err(err).Msg(initErrMsg)
				continue
			}
			collectors = append(collectors, c)

		case diskName:
			c, err := NewDiskCollector(path.Join(defaults.EtcPath, cfgBase))
			if err != nil {
				l.Error().Str("name", name).Err(err).Msg(initErrMsg)
				continue
			}
			collectors = append(collectors, c)

		case fsName:
			c, err := NewFSCollector(path.Join(defaults.EtcPath, cfgBase))
			if err != nil {
				l.Error().Str("name", name).Err(err).Msg(initErrMsg)
				continue
			}
			collectors = append(collectors, c)

		case loadName:
			c, err := NewLoadCollector(path.Join(defaults.EtcPath, cfgBase))
			if err != nil {
				l.Error().Str("name", name).Err(err).Msg(initErrMsg)
				continue
			}
			collectors = append(collectors, c)

		case vmName:
			c, err := NewVMCollector(path.Join(defaults.EtcPath, cfgBase))
			if err != nil {
				l.Error().Str("name", name).Err(err).Msg(initErrMsg)
				continue
			}
			collectors = append(collectors, c)

		case ifName:
			c, err := NewNetIFCollector(path.Join(defaults.EtcPath, cfgBase))
			if err != nil {
				l.Error().Str("name", name).Err(err).Msg(initErrMsg)
				continue
			}
			collectors = append(collectors, c)

		case protoName:
			c, err := NewNetProtoCollector(path.Join(defaults.EtcPath, cfgBase))
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
