// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build linux

package procfs

import (
	"fmt"
	"path"
	"regexp"
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

// pfscommon defines ProcFS metrics common elements
type pfscommon struct {
	id                  string          // OPT id of the collector (used as metric name prefix)
	pkgID               string          // package prefix used for logging and errors
	procFSPath          string          // OPT procfs mount point path
	file                string          // the file in procfs
	lastEnd             time.Time       // last collection end time
	lastError           string          // last collection error
	lastMetrics         cgm.Metrics     // last metrics collected
	lastRunDuration     time.Duration   // last collection duration
	lastStart           time.Time       // last collection start time
	logger              zerolog.Logger  // collector logging instance
	metricDefaultActive bool            // OPT default status for metrics NOT explicitly in metricStatus
	metricNameChar      string          // OPT character(s) used as replacement for metricNameRegex
	metricNameRegex     *regexp.Regexp  // OPT regex for cleaning names, may be overridden in config
	metricStatus        map[string]bool // OPT list of metrics and whether they should be collected or not
	running             bool            // is collector currently running
	runTTL              time.Duration   // OPT ttl for collectors (default is for every request)
	baseTags            tags.Tags
	sync.Mutex
}

const (
	PROCFS_PREFIX       = "procfs/"
	PKG_NAME            = "builtins.linux.procfs"
	PROC_FS_PATH        = "/proc"
	CPU_NAME            = "cpu"
	DISKSTATS_NAME      = "diskstats"
	IF_NAME             = "if"
	LOADAVG_NAME        = "loadavg"
	VM_NAME             = "vm"
	metricNameSeparator = "`"        // character used to separate parts of metric names
	metricStatusEnabled = "enabled"  // setting string indicating metrics should be made 'active'
	regexPat            = `^(?:%s)$` // fmt pattern used compile include/exclude regular expressions
)

var (
	defaultExcludeRegex = regexp.MustCompile(fmt.Sprintf(regexPat, ""))
	defaultIncludeRegex = regexp.MustCompile(fmt.Sprintf(regexPat, ".+"))
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

	collectors := make([]collector.Collector, 0, len(enbledCollectors))
	initErrMsg := "initializing builtin collector"
	for _, name := range enbledCollectors {
		if !strings.HasPrefix(name, PROCFS_PREFIX) {
			continue
		}
		name = strings.Replace(name, PROCFS_PREFIX, "", -1)
		cfgBase := "procfs_" + name + "_collector"
		switch name {
		case CPU_NAME:
			c, err := NewCPUCollector(path.Join(defaults.EtcPath, cfgBase), PROC_FS_PATH)
			if err != nil {
				l.Error().Str("name", name).Err(err).Msg(initErrMsg)
				continue
			}
			collectors = append(collectors, c)

		case DISKSTATS_NAME:
			c, err := NewDiskstatsCollector(path.Join(defaults.EtcPath, cfgBase), PROC_FS_PATH)
			if err != nil {
				l.Error().Str("name", name).Err(err).Msg(initErrMsg)
				continue
			}
			collectors = append(collectors, c)

		case IF_NAME:
			c, err := NewIFCollector(path.Join(defaults.EtcPath, cfgBase), PROC_FS_PATH)
			if err != nil {
				l.Error().Str("name", name).Err(err).Msg(initErrMsg)
				continue
			}
			collectors = append(collectors, c)

		case LOADAVG_NAME:
			c, err := NewLoadavgCollector(path.Join(defaults.EtcPath, cfgBase), PROC_FS_PATH)
			if err != nil {
				l.Error().Str("name", name).Err(err).Msg(initErrMsg)
				continue
			}
			collectors = append(collectors, c)

		case VM_NAME:
			c, err := NewVMCollector(path.Join(defaults.EtcPath, cfgBase), PROC_FS_PATH)
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
