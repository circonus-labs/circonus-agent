// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package prometheus

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
	metricNameRegex     *regexp.Regexp  // OPT regex for cleaning names, may be overridden in config
	metricStatus        map[string]bool // OPT list of metrics and whether they should be collected or not
	running             bool            // is collector currently running
	runTTL              time.Duration   // OPT ttl for collector (default is for every request)
	include             *regexp.Regexp
	exclude             *regexp.Regexp
	baseTags            []string
	sync.Mutex
}

// promOptions defines what elements can be overridden in a config file
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

// New creates new prom collector
func New(cfgBaseName string) (collector.Collector, error) {
	c := Prom{
		pkgID:               "builtins.prometheus",
		metricStatus:        map[string]bool{},
		metricDefaultActive: true,
		include:             defaultIncludeRegex,
		exclude:             defaultExcludeRegex,
		metricNameRegex:     regexp.MustCompile("[\r\n\"']"), // used to strip unwanted characters
		baseTags:            tags.GetBaseTags(),
	}

	c.logger = log.With().Str("pkg", c.pkgID).Logger()

	// Prom is a special builtin, it requires a configuration file,
	// it obviously would not work if there are no urls from which
	// to pull metrics. The default config is a file named
	// prometheus_collector.(json|toml|yaml) located in the agent's
	// default etc path. (e.g. /opt/circonus/agent/etc/prometheus_collector.yaml)
	if cfgBaseName == "" {
		cfgBaseName = path.Join(defaults.EtcPath, "prometheus_collector")
	}

	var opts promOptions
	err := config.LoadConfigFile(cfgBaseName, &opts)
	if err != nil {
		return nil, errors.Wrapf(err, "%s config", c.pkgID)
	}

	c.logger.Debug().Str("base", cfgBaseName).Interface("config", opts).Msg("loaded config")

	if len(opts.URLs) == 0 {
		return nil, errors.New("'urls' is REQUIRED in configuration")
	}
	for i, u := range opts.URLs {
		if u.ID == "" {
			c.logger.Warn().Int("item", i).Interface("url", u).Msg("invalid id (empty), ignoring URL entry")
			continue
		}
		if u.URL == "" {
			c.logger.Warn().Int("item", i).Interface("url", u).Msg("invalid URL (empty), ignoring URL entry")
			continue
		}
		_, err := url.Parse(u.URL)
		if err != nil {
			c.logger.Warn().Err(err).Int("item", i).Interface("url", u).Msg("invalid URL, ignoring URL entry")
			continue
		}
		if u.TTL != "" {
			ttl, err := time.ParseDuration(u.TTL)
			if err != nil {
				c.logger.Warn().Err(err).Int("item", i).Interface("url", u).Msg("invalid TTL, ignoring")
			} else {
				u.uttl = ttl
			}
		}
		if u.uttl == time.Duration(0) {
			u.uttl = 30 * time.Second
		}
		c.logger.Debug().Int("item", i).Interface("url", u).Msg("enabling prom collection URL")
		c.urls = append(c.urls, u)
	}

	if opts.IncludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, opts.IncludeRegex))
		if err != nil {
			return nil, errors.Wrapf(err, "%s compiling include regex", c.pkgID)
		}
		c.include = rx
	}

	if opts.ExcludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, opts.ExcludeRegex))
		if err != nil {
			return nil, errors.Wrapf(err, "%s compiling exclude regex", c.pkgID)
		}
		c.exclude = rx
	}

	if len(opts.MetricsEnabled) > 0 {
		for _, name := range opts.MetricsEnabled {
			c.metricStatus[name] = true
		}
	}
	if len(opts.MetricsDisabled) > 0 {
		for _, name := range opts.MetricsDisabled {
			c.metricStatus[name] = false
		}
	}

	if opts.MetricsDefaultStatus != "" {
		if ok, _ := regexp.MatchString(`^(enabled|disabled)$`, strings.ToLower(opts.MetricsDefaultStatus)); ok {
			c.metricDefaultActive = strings.ToLower(opts.MetricsDefaultStatus) == metricStatusEnabled
		} else {
			return nil, errors.Errorf("%s invalid metric default status (%s)", c.pkgID, opts.MetricsDefaultStatus)
		}
	}

	if opts.RunTTL != "" {
		dur, err := time.ParseDuration(opts.RunTTL)
		if err != nil {
			return nil, errors.Wrapf(err, "%s parsing run_ttl", c.pkgID)
		}
		c.runTTL = dur
	}

	return &c, nil
}

// Collect returns collector metrics
func (c *Prom) Collect() error {
	metrics := cgm.Metrics{}
	c.Lock()

	if c.running {
		c.logger.Warn().Msg(collector.ErrAlreadyRunning.Error())
		c.Unlock()
		return collector.ErrAlreadyRunning
	}

	if c.runTTL > time.Duration(0) {
		if time.Since(c.lastEnd) < c.runTTL {
			c.logger.Warn().Msg(collector.ErrTTLNotExpired.Error())
			c.Unlock()
			return collector.ErrTTLNotExpired
		}
	}

	c.running = true
	c.lastStart = time.Now()
	c.Unlock()

	for _, u := range c.urls {
		c.logger.Debug().Str("id", u.ID).Str("url", u.URL).Msg("prom fetch request")
		err := c.fetchPromMetrics(u, &metrics)
		if err != nil {
			c.logger.Error().Err(err).Interface("url", u).Msg("fetching prom metrics")
		}
	}

	c.setStatus(metrics, nil)
	return nil
}

func (c *Prom) fetchPromMetrics(u URLDef, metrics *cgm.Metrics) error {
	req, err := http.NewRequest("GET", u.URL, nil)
	if err != nil {
		return err
	}

	var ctx context.Context
	var cancel context.CancelFunc

	if u.uttl > time.Duration(0) {
		ctx, cancel = context.WithTimeout(context.Background(), u.uttl)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}

	req = req.WithContext(ctx)
	defer cancel()

	err = c.httpDoRequest(ctx, req, func(resp *http.Response, err error) error {
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		// validate response headers

		return c.parse(u.ID, resp.Body, metrics)
	})

	return err
}

func (c *Prom) httpDoRequest(ctx context.Context, req *http.Request, respHandler func(*http.Response, error) error) error {
	client := &http.Client{Transport: &http.Transport{DisableCompression: false, DisableKeepAlives: true, MaxIdleConnsPerHost: 1}}
	ec := make(chan error, 1)

	go func() { ec <- respHandler(client.Do(req)) }()

	select {
	case <-ctx.Done():
		<-ec
		return ctx.Err()
	case err := <-ec:
		return err
	}
}

func (c *Prom) parse(id string, data io.ReadCloser, metrics *cgm.Metrics) error {
	var parser expfmt.TextParser

	// formats supported from https://prometheus.io/docs/instrumenting/exposition_formats/

	metricFamilies, err := parser.TextToMetricFamilies(data)
	if err != nil {
		return err
	}

	pfx := id
	for mn, mf := range metricFamilies {
		for _, m := range mf.Metric {
			metricName := mn
			tags := c.getLabels(m)
			if mf.GetType() == dto.MetricType_SUMMARY {
				c.addMetric(metrics, pfx, metricName+"_count", tags, "n", float64(m.GetSummary().GetSampleCount()))
				c.addMetric(metrics, pfx, metricName+"_sum", tags, "n", float64(m.GetSummary().GetSampleSum()))
				for qn, qv := range c.getQuantiles(m) {
					c.addMetric(metrics, pfx, metricName+"_"+qn, tags, "n", qv)
				}
			} else if mf.GetType() == dto.MetricType_HISTOGRAM {
				c.addMetric(metrics, pfx, metricName+"_count", tags, "n", float64(m.GetHistogram().GetSampleCount()))
				c.addMetric(metrics, pfx, metricName+"_sum", tags, "n", float64(m.GetHistogram().GetSampleSum()))
				for bn, bv := range c.getBuckets(m) {
					c.addMetric(metrics, pfx, metricName+"_"+bn, tags, "n", bv)
				}
			} else {
				if m.Gauge != nil {
					if m.GetGauge().Value != nil {
						c.addMetric(metrics, pfx, metricName, tags, "n", *m.GetGauge().Value)
					}
				} else if m.Counter != nil {
					if m.GetCounter().Value != nil {
						c.addMetric(metrics, pfx, metricName, tags, "n", *m.GetCounter().Value)
					}
				} else if m.Untyped != nil {
					if m.GetUntyped().Value != nil {
						c.addMetric(metrics, pfx, metricName, tags, "n", *m.GetUntyped().Value)
					}
				}
			}
		}
	}

	return nil
}

func (c *Prom) getLabels(m *dto.Metric) tags.Tags {
	// Need to use cgm.Tags format and return a converted stream tags string
	labels := []string{}

	for _, label := range m.Label {
		if label.Name != nil && label.Value != nil {
			ln := c.metricNameRegex.ReplaceAllString(*label.Name, "")
			lv := c.metricNameRegex.ReplaceAllString(*label.Value, "")
			labels = append(labels, ln+tags.Delimiter+lv) // stream tags take form cat:val
		}
	}

	if len(labels) > 0 {
		tagList := make([]string, 0, len(c.baseTags)+len(labels))
		tagList = append(tagList, c.baseTags...)
		tagList = append(tagList, labels...)
		tags := tags.FromList(tagList)
		return tags
	}

	return tags.Tags{}
}

func (c *Prom) getQuantiles(m *dto.Metric) map[string]float64 {
	ret := make(map[string]float64)
	for _, q := range m.GetSummary().Quantile {
		if q.Value != nil && !math.IsNaN(*q.Value) {
			ret[fmt.Sprint(*q.Quantile)] = *q.Value
		}
	}
	return ret
}

func (c *Prom) getBuckets(m *dto.Metric) map[string]uint64 {
	ret := make(map[string]uint64)
	for _, b := range m.GetHistogram().Bucket {
		if b.CumulativeCount != nil {
			ret[fmt.Sprint(*b.UpperBound)] = *b.CumulativeCount
		}
	}
	return ret
}
