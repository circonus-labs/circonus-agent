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
	stdlog "log"
	"math"
	"regexp"
	"strings"

	"github.com/circonus-labs/circonus-agent/internal/config"
	cgm "github.com/circonus-labs/circonus-gometrics"
	"github.com/pkg/errors"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func initCGM() error {
	metricsmu.Lock()
	defer metricsmu.Unlock()

	if metrics != nil {
		return nil
	}

	cmc := &cgm.Config{
		Debug: viper.GetBool(config.KeyDebugCGM),
		Log:   stdlog.New(log.With().Str("pkg", "promrecv").Logger(), "", 0),
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

	return nil
}

// Flush returns current metrics
func Flush() *cgm.Metrics {
	initCGM()
	metricsmu.Lock()
	defer metricsmu.Unlock()

	return metrics.FlushMetrics()
}

// Parse handles incoming PUT/POST requests
func Parse(data io.ReadCloser) error {
	initCGM()
	metricsmu.Lock()
	defer metricsmu.Unlock()

	var parser expfmt.TextParser

	// formats supported from https://prometheus.io/docs/instrumenting/exposition_formats/

	metricFamilies, err := parser.TextToMetricFamilies(data)
	if err != nil {
		return err
	}

	for mn, mf := range metricFamilies {
		metricName := id + metricNameSeparator + nameCleanerRx.ReplaceAllString(mn, "")
		for _, m := range mf.Metric {
			labels := getLabels(m)
			if len(labels) > 0 {
				metricName += metricNameSeparator + strings.Join(labels, metricNameSeparator)
			}
			if mf.GetType() == dto.MetricType_SUMMARY {
				metrics.Gauge(metricName+metricNameSeparator+"count", float64(m.GetSummary().GetSampleCount()))
				metrics.Gauge(metricName+metricNameSeparator+"sum", float64(m.GetSummary().GetSampleSum()))
				for qn, qv := range getQuantiles(m) {
					metrics.Gauge(metricName+metricNameSeparator+qn, qv)
				}
			} else if mf.GetType() == dto.MetricType_HISTOGRAM {
				metrics.Gauge(metricName+metricNameSeparator+"count", float64(m.GetHistogram().GetSampleCount()))
				metrics.Gauge(metricName+metricNameSeparator+"sum", float64(m.GetHistogram().GetSampleSum()))
				for bn, bv := range getBuckets(m) {
					metrics.Gauge(metricName+metricNameSeparator+bn, bv)
				}
			} else {
				if m.Gauge != nil {
					if m.GetGauge().Value != nil {
						metrics.Gauge(metricName, *m.GetGauge().Value)
					}
				} else if m.Counter != nil {
					if m.GetCounter().Value != nil {
						metrics.Gauge(metricName, *m.GetCounter().Value)
					}
				} else if m.Untyped != nil {
					if m.GetUntyped().Value != nil {
						metrics.Gauge(metricName, *m.GetUntyped().Value)
					}
				}
			}
		}
	}

	return nil
}

func getLabels(m *dto.Metric) []string {
	ret := []string{}
	for _, label := range m.Label {
		if label.Name != nil && label.Value != nil {
			ln := nameCleanerRx.ReplaceAllString(*label.Name, "")
			lv := nameCleanerRx.ReplaceAllString(*label.Value, "")
			ret = append(ret, ln+"="+lv)
		}
	}
	return ret
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
