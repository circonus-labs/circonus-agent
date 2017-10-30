// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package procfs

import (
	"fmt"
	"regexp"
	"sync"
	"time"

	cgm "github.com/circonus-labs/circonus-gometrics"
	"github.com/rs/zerolog"
)

// pfscommon defines ProcFS metrics common elements
type pfscommon struct {
	file                string          // OPT proc file path, may be overriden in config file
	id                  string          // OPT id of the collector (used as metric name prefix)
	lastEnd             time.Time       // last collection end time
	lastError           string          // last collection error
	lastMetrics         cgm.Metrics     // last metrics collected
	lastRunDuration     time.Duration   // last collection duration
	lastStart           time.Time       // last collection start time
	logger              zerolog.Logger  // collector logging instance
	metricDefaultActive bool            // OPT default status for metrics NOT explicitly in metricStatus, may be overriden in config file
	metricNameChar      string          // OPT character(s) used as replacement for metricNameRegex, may be overriden in config
	metricNameRegex     *regexp.Regexp  // OPT regex for cleaning names, may be overriden in config
	metricStatus        map[string]bool // OPT list of metrics and whether they should be collected or not, may be overriden in config file
	running             bool            // is collector currently running
	runTTL              time.Duration   // OPT ttl for collections, may be overriden in config file (default is for every request)
	sync.Mutex
}

const (
	metricNameSeparator = "`"        // character used to separate parts of metric names
	metricStatusEnabled = "enabled"  // setting string indicating metrics should be made 'active'
	regexPat            = `^(?:%s)$` // fmt pattern used compile include/exclude regular expressions
)

var (
	defaultExcludeRegex = regexp.MustCompile(fmt.Sprintf(regexPat, ""))
	defaultIncludeRegex = regexp.MustCompile(fmt.Sprintf(regexPat, ".+"))
)
