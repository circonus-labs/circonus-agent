// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package generic provides more cross-platform support for collecting basic system metrics
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
	NamePrefix  = "generic/"
	PackageName = "builtins.generic"
	NameCPU     = "cpu"
	NameDisk    = "disk"
	NameFS      = "fs"
	NameLoad    = "load"
	NameVM      = "vm"
	NameIF      = "if"
	NameProto   = "proto"
	regexPat    = `^(?:%s)$` // fmt pattern used compile include/exclude regular expressions
)

var (
	defaultExcludeRegex = regexp.MustCompile(fmt.Sprintf(regexPat, ""))
	defaultIncludeRegex = regexp.MustCompile(fmt.Sprintf(regexPat, ".+"))
)

// New creates new PSUtil collector
func New() ([]collector.Collector, error) {
	none := []collector.Collector{}

	l := log.With().Str("pkg", PackageName).Logger()

	enbledCollectors := viper.GetStringSlice(config.KeyCollectors)
	if len(enbledCollectors) == 0 {
		l.Info().Msg("no builtin collectors enabled")
		return none, nil
	}

	collectors := make([]collector.Collector, 0, len(enbledCollectors))
	initErrMsg := "initializing builtin collector"
	for _, name := range enbledCollectors {
		if !strings.HasPrefix(name, NamePrefix) {
			continue
		}
		name = strings.ReplaceAll(name, NamePrefix, "")
		cfgBase := "generic_" + name + "_collector"
		switch name {
		case NameCPU:
			c, err := NewCPUCollector(path.Join(defaults.EtcPath, cfgBase), l)
			if err != nil {
				l.Error().Str("name", name).Err(err).Msg(initErrMsg)
				continue
			}
			collectors = append(collectors, c)

		case NameDisk:
			c, err := NewDiskCollector(path.Join(defaults.EtcPath, cfgBase), l)
			if err != nil {
				l.Error().Str("name", name).Err(err).Msg(initErrMsg)
				continue
			}
			collectors = append(collectors, c)

		case NameFS:
			c, err := NewFSCollector(path.Join(defaults.EtcPath, cfgBase), l)
			if err != nil {
				l.Error().Str("name", name).Err(err).Msg(initErrMsg)
				continue
			}
			collectors = append(collectors, c)

		case NameLoad:
			c, err := NewLoadCollector(path.Join(defaults.EtcPath, cfgBase), l)
			if err != nil {
				l.Error().Str("name", name).Err(err).Msg(initErrMsg)
				continue
			}
			collectors = append(collectors, c)

		case NameIF:
			c, err := NewNetIFCollector(path.Join(defaults.EtcPath, cfgBase), l)
			if err != nil {
				l.Error().Str("name", name).Err(err).Msg(initErrMsg)
				continue
			}
			collectors = append(collectors, c)

		case NameProto:
			c, err := NewNetProtoCollector(path.Join(defaults.EtcPath, cfgBase), l)
			if err != nil {
				l.Error().Str("name", name).Err(err).Msg(initErrMsg)
				continue
			}
			collectors = append(collectors, c)

		case NameVM:
			c, err := NewVMCollector(path.Join(defaults.EtcPath, cfgBase), l)
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
