// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

//go:build windows
// +build windows

package wmi

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	// "github.com/StackExchange/wmi".
	"github.com/bi-zone/wmi"
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// wmicommon defines WMI metrics common elements.
type wmicommon struct {
	id              string         // id of the collector (used as metric name prefix)
	pkgID           string         // package prefix used for logging and errors
	lastError       string         // last collection error
	metricNameChar  string         // OPT character(s) used as replacement for metricNameRegex, may be overridden in config
	baseTags        tags.Tags      // base tags
	lastEnd         time.Time      // last collection end time
	lastMetrics     cgm.Metrics    // last metrics collected
	lastStart       time.Time      // last collection start time
	metricNameRegex *regexp.Regexp // OPT regex for cleaning names, may be overridden in config
	logger          zerolog.Logger // collector logging instance
	lastRunDuration time.Duration  // last collection duration
	runTTL          time.Duration  // OPT ttl for collections, may be overridden in config file (default is for every request)
	running         bool           // is collector currently running
	sync.Mutex
}

const (
	wmiPrefix           = "wmi/"
	pkgName             = "builtins.windows.wmi"
	defaultMetricChar   = "_"        // character used to replace invalid characters in metric name
	metricNameSeparator = "`"        // character used to separate parts of metric names
	regexPat            = `^(?:%s)$` // fmt pattern used compile include/exclude regular expressions
	totalName           = "_Total"   // value of the Name field for 'totals'
)

var (
	errInvalidMetric       = fmt.Errorf("invalid metric, nil")
	errInvalidMetricNoName = fmt.Errorf("invalid metric, no name")
	errInvalidMetricNoType = fmt.Errorf("invalid metric, no type")
	defaultExcludeRegex    = regexp.MustCompile(fmt.Sprintf(regexPat, ``))
	defaultIncludeRegex    = regexp.MustCompile(fmt.Sprintf(regexPat, `.+`))
	defaultMetricNameRegex = regexp.MustCompile(`[^a-zA-Z0-9.-_:` + metricNameSeparator + `]`)
)

func initialize() error {
	// This initialization prevents a memory leak on WMF 5+. See
	// https://github.com/martinlindhe/wmi_exporter/issues/77 and
	// linked issues for details.
	// s, err := wmi.InitializeSWbemServices(wmi.DefaultClient)
	s, err := wmi.NewSWbemServices()
	if err != nil {
		return fmt.Errorf("init SWbemSvc: %w", err)
	}
	wmi.DefaultClient.SWbemServicesClient = s
	return nil
}

// New creates new WMI collector.
func New() ([]collector.Collector, error) {
	none := []collector.Collector{}
	l := log.With().Str("pkg", pkgName).Logger()

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
		if !strings.HasPrefix(name, wmiPrefix) {
			continue
		}
		name = strings.ReplaceAll(name, wmiPrefix, "")
		cfgBase := "wmi_" + name + "_collector"
		switch name {
		case "cache":
			c, err := NewCacheCollector(path.Join(defaults.EtcPath, cfgBase))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "disk":
			c, err := NewDiskCollector(path.Join(defaults.EtcPath, cfgBase))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "memory":
			c, err := NewMemoryCollector(path.Join(defaults.EtcPath, cfgBase))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "interface":
			c, err := NewNetInterfaceCollector(path.Join(defaults.EtcPath, cfgBase))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "ip":
			c, err := NewNetIPCollector(path.Join(defaults.EtcPath, cfgBase))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "tcp":
			c, err := NewNetTCPCollector(path.Join(defaults.EtcPath, cfgBase))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "udp":
			c, err := NewNetUDPCollector(path.Join(defaults.EtcPath, cfgBase))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "objects":
			c, err := NewObjectsCollector(path.Join(defaults.EtcPath, cfgBase))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "paging_file":
			c, err := NewPagingFileCollector(path.Join(defaults.EtcPath, cfgBase))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "processes":
			c, err := NewProcessesCollector(path.Join(defaults.EtcPath, cfgBase))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "processor":
			c, err := NewProcessorCollector(path.Join(defaults.EtcPath, cfgBase))
			if err != nil {
				logError(name, err)
				continue
			}
			collectors = append(collectors, c)

		case "system":
			c, err := NewSystemCollector(path.Join(defaults.EtcPath, cfgBase))
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

func done(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}

}
