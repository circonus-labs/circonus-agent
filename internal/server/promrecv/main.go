// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// thanks to https://github.com/influxdata/telegraf/blob/release-1.2/plugins/inputs/prometheus/parser.go
// and https://github.com/prometheus/common/tree/master/expfmt

package promrecv

import (
	"fmt"
	"io"
	"math"
	"regexp"
	"sync"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

var (
	id                  string
	baseTags            []string
	nameCleanerRx       *regexp.Regexp
	metricNameSeparator = "`"
	metricsmu           sync.Mutex
	metrics             *cgm.CirconusMetrics
	parseRx             *regexp.Regexp
	logger              = log.With().Str("pkg", "promrecv").Logger()
)

// logshim is used to satisfy apiclient Logger interface (avoiding ptr receiver issue)
type logshim struct {
	logh zerolog.Logger
}

func (l logshim) Printf(fmt string, v ...interface{}) {
	l.logh.Printf(fmt, v...)
}

func initCGM() error {
	metricsmu.Lock()
	defer metricsmu.Unlock()

	if metrics != nil {
		return nil
	}

	cmc := &cgm.Config{
		Debug: viper.GetBool(config.KeyDebugCGM),
		Log:   logshim{logh: log.With().Str("pkg", "cgm.promrecv").Logger()},
	}
	// put cgm into manual mode (no interval, no api key, invalid submission url)
	cmc.Interval = "0"                            // disable automatic flush
	cmc.CheckManager.Check.SubmissionURL = "none" // disable check management (create/update)

	hm, err := cgm.NewCirconusMetrics(cmc)
	if err != nil {
		return errors.Wrap(err, "prom receiver cgm")
	}

	metrics = hm

	// inintialize any options for the receiver
	id = "prom"                                      // metric name (group) prefix to be used
	nameCleanerRx = regexp.MustCompile("[\r\n\"'`]") // used to strip unwanted characters

	baseTags = tags.GetBaseTags()

	return nil
}

// Flush returns current metrics
func Flush() *cgm.Metrics {
	initCGM()

	return metrics.FlushMetrics()
}

// Parse handles incoming PUT/POST requests
func Parse(data io.ReadCloser) error {
	initCGM()

	var parser expfmt.TextParser

	// formats supported from https://prometheus.io/docs/instrumenting/exposition_formats/

	metricFamilies, err := parser.TextToMetricFamilies(data)
	if err != nil {
		return err
	}

	for mn, mf := range metricFamilies {
		for _, m := range mf.Metric {
			metricName := id + metricNameSeparator + nameCleanerRx.ReplaceAllString(mn, "")
			tags := getLabels(m)
			if mf.GetType() == dto.MetricType_SUMMARY {
				metrics.Gauge(metricName+"_count", float64(m.GetSummary().GetSampleCount()))
				metrics.Gauge(metricName+"_sum", float64(m.GetSummary().GetSampleSum()))
				for qn, qv := range getQuantiles(m) {
					metrics.GaugeWithTags(metricName+"_"+qn, tags, qv)
				}
			} else if mf.GetType() == dto.MetricType_HISTOGRAM {
				metrics.Gauge(metricName+"_count", float64(m.GetHistogram().GetSampleCount()))
				metrics.Gauge(metricName+"_sum", float64(m.GetHistogram().GetSampleSum()))
				for bn, bv := range getBuckets(m) {
					metrics.GaugeWithTags(metricName+"_"+bn, tags, bv)
				}
			} else {
				if m.Gauge != nil {
					if m.GetGauge().Value != nil {
						metrics.GaugeWithTags(metricName, tags, *m.GetGauge().Value)
					}
				} else if m.Counter != nil {
					if m.GetCounter().Value != nil {
						metrics.GaugeWithTags(metricName, tags, *m.GetCounter().Value)
					}
				} else if m.Untyped != nil {
					if m.GetUntyped().Value != nil {
						metrics.GaugeWithTags(metricName, tags, *m.GetUntyped().Value)
					}
				}
			}
		}
	}

	return nil
}

func getLabels(m *dto.Metric) tags.Tags {
	labels := make([]string, 0, len(m.Label))
	for _, label := range m.Label {
		if label.Name != nil && label.Value != nil {
			ln := nameCleanerRx.ReplaceAllString(*label.Name, "")
			lv := nameCleanerRx.ReplaceAllString(*label.Value, "")
			labels = append(labels, ln+tags.Delimiter+lv) // stream tags take form cat:val
		}
	}

	if len(labels) > 0 {
		tagList := make([]string, 0, len(baseTags)+len(labels))
		tagList = append(tagList, baseTags...)
		tagList = append(tagList, labels...)
		tags := tags.FromList(tagList)
		return tags
	}

	return tags.Tags{}
}

func getQuantiles(m *dto.Metric) map[string]float64 {
	ret := make(map[string]float64)
	for _, q := range m.GetSummary().Quantile {
		if q.Value != nil && !math.IsNaN(*q.Value) {
			ret[fmt.Sprint(*q.Quantile)] = *q.Value
		}
	}
	return ret
}

func getBuckets(m *dto.Metric) map[string]uint64 {
	ret := make(map[string]uint64)
	for _, b := range m.GetHistogram().Bucket {
		if b.CumulativeCount != nil {
			ret[fmt.Sprint(*b.UpperBound)] = *b.CumulativeCount
		}
	}
	return ret
}
