// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package prometheus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// URLDef defines a url to fetch text formatted prom metrics from.
type URLDef struct {
	ID   string `json:"id" toml:"id" yaml:"id"`
	URL  string `json:"url" toml:"url" yaml:"url"`
	TTL  string `json:"ttl" toml:"ttl" yaml:"ttl"`
	uttl time.Duration
}

// Prom defines prom collector.
type Prom struct {
	pkgID           string         // package prefix used for logging and errors
	lastError       string         // last collection error
	baseTags        []string       // base tags
	urls            []URLDef       // prom URLs to collect metric from
	lastEnd         time.Time      // last collection end time
	lastMetrics     cgm.Metrics    // last metrics collected
	lastStart       time.Time      // last collection start time
	metricNameRegex *regexp.Regexp // OPT regex for cleaning names, may be overridden in config
	logger          zerolog.Logger // collector logging instance
	lastRunDuration time.Duration  // last collection duration
	runTTL          time.Duration  // OPT ttl for collector (default is for every request)
	running         bool           // is collector currently running
	sync.Mutex
}

// promOptions defines what elements can be overridden in a config file.
type promOptions struct {
	RunTTL string   `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
	URLs   []URLDef `json:"urls" toml:"urls" yaml:"urls"`
}

var (
	errInvalidMetric       = fmt.Errorf("invalid metric, nil")
	errInvalidMetricNoName = fmt.Errorf("invalid metric, no name")
	errInvalidMetricNoType = fmt.Errorf("invalid metric, no type")
	errInvalidURLs         = fmt.Errorf("'urls' is REQUIRED in configuration")
)

// New creates new prom collector.
func New(cfgBaseName string) (collector.Collector, error) {
	c := Prom{
		pkgID:           "builtins.prometheus",
		metricNameRegex: regexp.MustCompile("[\r\n\"']"), // used to strip unwanted characters
		baseTags:        tags.GetBaseTags(),
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

	// a configuration file being found is what enables this plugin
	var opts promOptions
	err := config.LoadConfigFile(cfgBaseName, &opts)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil // if none found, return nothing
		}
		return nil, fmt.Errorf("%s config: %w", c.pkgID, err)
	}

	c.logger.Debug().Str("base", cfgBaseName).Interface("config", opts).Msg("loaded config")

	if len(opts.URLs) == 0 {
		return nil, errInvalidURLs
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

	if opts.RunTTL != "" {
		dur, err := time.ParseDuration(opts.RunTTL)
		if err != nil {
			return nil, fmt.Errorf("%s parsing run_ttl: %w", c.pkgID, err)
		}
		c.runTTL = dur
	}

	return &c, nil
}

// Collect returns collector metrics.
func (c *Prom) Collect(ctx context.Context) error {
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
		err := c.fetchPromMetrics(ctx, u, &metrics)
		if err != nil {
			c.logger.Error().Err(err).Interface("url", u).Msg("fetching prom metrics")
		}
	}

	c.setStatus(metrics, nil)
	return nil
}

func (c *Prom) fetchPromMetrics(pctx context.Context, u URLDef, metrics *cgm.Metrics) error {
	req, err := http.NewRequest("GET", u.URL, nil)
	if err != nil {
		return fmt.Errorf("prepare reqeust: %w", err)
	}

	var ctx context.Context
	var cancel context.CancelFunc

	if u.uttl > time.Duration(0) {
		ctx, cancel = context.WithTimeout(pctx, u.uttl)
	} else {
		ctx, cancel = context.WithCancel(pctx)
	}

	req = req.WithContext(ctx)
	defer cancel()

	ec := make(chan error, 1)

	go func() {
		client := &http.Client{
			Transport: &http.Transport{
				DisableCompression:  false,
				DisableKeepAlives:   true,
				MaxIdleConnsPerHost: 1,
			},
		}
		resp, err := client.Do(req)
		if err != nil {
			ec <- err
			return
		}
		defer resp.Body.Close()
		ec <- c.parse(u.ID, resp.Body, metrics)
	}()

	select {
	case <-ctx.Done():
		<-ec
		return ctx.Err() //nolint:wrapcheck
	case err := <-ec:
		return err
	}
}

func (c *Prom) parse(id string, data io.Reader, metrics *cgm.Metrics) error {
	var parser expfmt.TextParser

	// formats supported from https://prometheus.io/docs/instrumenting/exposition_formats/

	metricFamilies, err := parser.TextToMetricFamilies(data)
	if err != nil {
		return fmt.Errorf("parser - metric families: %w", err)
	}

	pfx := ""
	for mn, mf := range metricFamilies {
		for _, m := range mf.Metric {
			metricName := mn
			tags := c.getLabels(m)
			tags = append(tags, cgm.Tag{Category: "prom_id", Value: id})
			switch mf.GetType() {
			case dto.MetricType_SUMMARY:
				_ = c.addMetric(metrics, pfx, metricName+"_count", tags, "n", float64(m.GetSummary().GetSampleCount()))
				_ = c.addMetric(metrics, pfx, metricName+"_sum", tags, "n", m.GetSummary().GetSampleSum())
				for qn, qv := range c.getQuantiles(m) {
					_ = c.addMetric(metrics, pfx, metricName+"_"+qn, tags, "n", qv)
				}
			case dto.MetricType_HISTOGRAM:
				_ = c.addMetric(metrics, pfx, metricName+"_count", tags, "n", float64(m.GetHistogram().GetSampleCount()))
				_ = c.addMetric(metrics, pfx, metricName+"_sum", tags, "n", m.GetHistogram().GetSampleSum())
				for bn, bv := range c.getBuckets(m) {
					_ = c.addMetric(metrics, pfx, metricName+"_"+bn, tags, "n", bv)
				}
			case dto.MetricType_COUNTER:
				if m.GetCounter().Value != nil {
					_ = c.addMetric(metrics, pfx, metricName, tags, "n", *m.GetCounter().Value)
				}
			case dto.MetricType_GAUGE:
				if m.GetGauge().Value != nil {
					_ = c.addMetric(metrics, pfx, metricName, tags, "n", *m.GetGauge().Value)
				}
			case dto.MetricType_UNTYPED:
				if m.GetUntyped().Value != nil {
					if *m.GetUntyped().Value == math.Inf(+1) {
						c.logger.Warn().Str("metric", metricName).Str("type", mf.GetType().String()).Str("value", (*m).GetUntyped().String()).Msg("cannot coerce +Inf to uint64")
						continue
					}
					_ = c.addMetric(metrics, pfx, metricName, tags, "n", *m.GetUntyped().Value)
				}
			case dto.MetricType_GAUGE_HISTOGRAM:
				// not currently supported
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

	return tags.FromList(c.baseTags)
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
