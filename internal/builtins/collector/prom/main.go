// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package prom

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	cgm "github.com/circonus-labs/circonus-gometrics"
	"github.com/pkg/errors"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/rs/zerolog/log"
)

// New creates new prom collector
func New(cfgBaseName string) (collector.Collector, error) {
	c := Prom{
		metricStatus:        map[string]bool{},
		metricDefaultActive: true,
		include:             defaultIncludeRegex,
		exclude:             defaultExcludeRegex,
		metricNameRegex:     regexp.MustCompile("[\r\n\"']"), // used to strip unwanted characters
	}
	c.pkgID = "builtins.promfetch"
	c.logger = log.With().Str("pkg", c.pkgID).Logger()

	// Prom is a special builtin, it requires a configuration file,
	// it does not work if there are no prom urls to pull metrics from.
	// default config is a file named promfetch.(json|toml|yaml) in the
	// agent's default etc path.
	if cfgBaseName == "" {
		cfgBaseName = path.Join(defaults.EtcPath, "prometheus_collector")
	}

	var opts promOptions
	err := config.LoadConfigFile(cfgBaseName, &opts)
	if err != nil {
		c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, errors.Wrapf(err, "%s config", c.pkgID)
	}

	c.logger.Debug().Str("base", cfgBaseName).Interface("config", opts).Msg("loaded config")

	if len(opts.URLs) == 0 {
		return nil, errors.New("'urls' is REQUIRED in configuration")
	}
	for i, u := range opts.URLs {
		if u.ID == "" {
			c.logger.Warn().Int("item", i).Interface("url", u).Msg("invalid id (empty), ignoring")
			continue
		}
		if u.URL == "" {
			c.logger.Warn().Int("item", i).Interface("url", u).Msg("invalid URL (empty), ignoring")
			continue
		}
		_, err := url.Parse(u.URL)
		if err != nil {
			c.logger.Warn().Err(err).Int("item", i).Interface("url", u).Msg("invalid URL, ignoring")
			continue
		}
		if u.TTL != "" {
			ttl, err := time.ParseDuration(u.TTL)
			if err != nil {
				c.logger.Warn().Err(err).Int("item", i).Interface("url", u).Msg("invalid TTL, ignoring")
			}
			u.uttl = ttl
		} else {
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
	ctx, _ := context.WithTimeout(context.Background(), u.uttl)

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
	tr := &http.Transport{DisableCompression: false, DisableKeepAlives: true, MaxIdleConnsPerHost: 1}
	client := &http.Client{Transport: tr}
	ec := make(chan error, 1)

	go func() { ec <- respHandler(client.Do(req)) }()

	select {
	case <-ctx.Done():
		tr.CancelRequest(req)
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
			labels := c.getLabels(m)
			if len(labels) > 0 {
				metricName += metricNameSeparator + strings.Join(labels, metricNameSeparator)
			}
			if mf.GetType() == dto.MetricType_SUMMARY {
				c.addMetric(metrics, pfx, metricName+"_count", "n", float64(m.GetSummary().GetSampleCount()))
				c.addMetric(metrics, pfx, metricName+"_sum", "n", float64(m.GetSummary().GetSampleSum()))
				for qn, qv := range c.getQuantiles(m) {
					c.addMetric(metrics, pfx, metricName+"_"+qn, "n", qv)
				}
			} else if mf.GetType() == dto.MetricType_HISTOGRAM {
				c.addMetric(metrics, pfx, metricName+"_count", "n", float64(m.GetHistogram().GetSampleCount()))
				c.addMetric(metrics, pfx, metricName+"_sum", "n", float64(m.GetHistogram().GetSampleSum()))
				for bn, bv := range c.getBuckets(m) {
					c.addMetric(metrics, pfx, metricName+"_"+bn, "n", bv)
				}
			} else {
				if m.Gauge != nil {
					if m.GetGauge().Value != nil {
						c.addMetric(metrics, pfx, metricName, "n", *m.GetGauge().Value)
					}
				} else if m.Counter != nil {
					if m.GetCounter().Value != nil {
						c.addMetric(metrics, pfx, metricName, "n", *m.GetCounter().Value)
					}
				} else if m.Untyped != nil {
					if m.GetUntyped().Value != nil {
						c.addMetric(metrics, pfx, metricName, "n", *m.GetUntyped().Value)
					}
				}
			}
		}
	}

	return nil
}

func (c *Prom) getLabels(m *dto.Metric) []string {
	ret := []string{}
	// sort for predictive metric names
	var keys []string
	labels := make(map[string]string)
	for _, label := range m.Label {
		if label.Name != nil && label.Value != nil {
			ln := c.metricNameRegex.ReplaceAllString(*label.Name, "")
			lv := c.metricNameRegex.ReplaceAllString(*label.Value, "")
			labels[ln] = lv
			keys = append(keys, ln)
		}
	}

	sort.Strings(keys)

	for _, label := range keys {
		ret = append(ret, label+"="+labels[label])
	}
	return ret
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
