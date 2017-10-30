// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build windows

package wmi

import (
	"path"
	"runtime"

	"github.com/StackExchange/wmi"
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func initialize() error {
	// This initialization prevents a memory leak on WMF 5+. See
	// https://github.com/martinlindhe/wmi_exporter/issues/77 and
	// linked issues for details.
	s, err := wmi.InitializeSWbemServices(wmi.DefaultClient)
	if err != nil {
		return err
	}
	wmi.DefaultClient.SWbemServicesClient = s
	return nil
}

// New creates new WMI collector
func New() ([]collector.Collector, error) {
	none := []collector.Collector{}
	l := log.With().Str("pkg", "builtins.wmi").Logger()

	if runtime.GOOS != "windows" {
		l.Warn().Msg("not windows, skipping wmi")
		return none, nil
	}

	if err := initialize(); err != nil {
		return none, err
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
		switch name {
		case "cache":
			c, err := NewCacheCollector(path.Join(defaults.EtcPath, "cache"))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "disk":
			c, err := NewDiskCollector(path.Join(defaults.EtcPath, "disk"))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "memory":
			c, err := NewMemoryCollector(path.Join(defaults.EtcPath, "memory"))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "interface":
			c, err := NewNetInterfaceCollector(path.Join(defaults.EtcPath, "interface"))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "ip":
			c, err := NewNetIPCollector(path.Join(defaults.EtcPath, "ip"))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "tcp":
			c, err := NewNetTCPCollector(path.Join(defaults.EtcPath, "tcp"))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "udp":
			c, err := NewNetUDPCollector(path.Join(defaults.EtcPath, "udp"))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "objects":
			c, err := NewObjectsCollector(path.Join(defaults.EtcPath, "objects"))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "paging_file":
			c, err := NewPagingFileCollector(path.Join(defaults.EtcPath, "paging_file"))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "processes":
			c, err := NewProcessesCollector(path.Join(defaults.EtcPath, "processes"))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "processor":
			c, err := NewProcessorCollector(path.Join(defaults.EtcPath, "processor"))
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
