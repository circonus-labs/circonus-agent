// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package prometheus

import (
	"fmt"
	"regexp"
	"sync"
	"time"

	cgm "github.com/circonus-labs/circonus-gometrics"
	"github.com/rs/zerolog"
)

// URLDef defines a url to fetch text formatted prom metrics from
type URLDef struct {
	ID   string `json:"id" toml:"id" yaml:"id"`
	URL  string `json:"url" toml:"url" yaml:"url"`
	TTL  string `json:"ttl" toml:"ttl" yaml:"ttl"`
	uttl time.Duration
}

// Prom defines prom collector
type Prom struct {
	pkgID               string          // package prefix used for logging and errors
	urls                []URLDef        // prom URLs to collect metric from
	lastEnd             time.Time       // last collection end time
	lastError           string          // last collection error
	lastMetrics         cgm.Metrics     // last metrics collected
	lastRunDuration     time.Duration   // last collection duration
	lastStart           time.Time       // last collection start time
	logger              zerolog.Logger  // collector logging instance
	metricDefaultActive bool            // OPT default status for metrics NOT explicitly in metricStatus
	metricNameRegex     *regexp.Regexp  // OPT regex for cleaning names, may be overriden in config
	metricStatus        map[string]bool // OPT list of metrics and whether they should be collected or not
	running             bool            // is collector currently running
	runTTL              time.Duration   // OPT ttl for collector (default is for every request)
	include             *regexp.Regexp
	exclude             *regexp.Regexp
	sync.Mutex
}

// promOptions defines what elements can be overriden in a config file
type promOptions struct {
	MetricsEnabled       []string `json:"metrics_enabled" toml:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsDisabled      []string `json:"metrics_disabled" toml:"metrics_disabled" yaml:"metrics_disabled"`
	MetricsDefaultStatus string   `json:"metrics_default_status" toml:"metrics_default_status" toml:"metrics_default_status"`
	RunTTL               string   `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
	IncludeRegex         string   `json:"include_regex" toml:"include_regex" yaml:"include_regex"`
	ExcludeRegex         string   `json:"exclude_regex" toml:"exclude_regex" yaml:"exclude_regex"`
	URLs                 []URLDef `json:"urls" toml:"urls" yaml:"urls"`
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
