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
	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

var (
	baseTags      []string
	nameCleanerRx *regexp.Regexp
	metricsmu     sync.Mutex
	metrics       *cgm.CirconusMetrics
	logger        = log.With().Str("pkg", "promrecv").Logger()
)

// logshim is used to satisfy apiclient Logger interface (avoiding ptr receiver issue).
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
		return fmt.Errorf("prom receiver cgm: %w", err)
	}

	metrics = hm

	// initialize any options for the receiver
	nameCleanerRx = regexp.MustCompile("[\r\n\"'`]") // used to strip unwanted characters

	baseTags = tags.GetBaseTags()
	baseTags = append(baseTags, []string{
		"source:" + release.NAME,
		"collector:promrecv",
	}...)

	return nil
}

// Flush returns current metrics.
func Flush() *cgm.Metrics {
	_ = initCGM()

	return metrics.FlushMetrics()
}

// Parse handles incoming PUT/POST requests.
func Parse(data io.Reader) error {
	if err := initCGM(); err != nil {
		return err
	}

	var parser expfmt.TextParser

	// formats supported from https://prometheus.io/docs/instrumenting/exposition_formats/

	metricFamilies, err := parser.TextToMetricFamilies(data)
	if err != nil {
		return fmt.Errorf("parse - text to metric families: %w", err)
	}

	for mn, mf := range metricFamilies {
		for _, m := range mf.GetMetric() {
			metricName := nameCleanerRx.ReplaceAllString(mn, "")
			tags := getLabels(m)
			switch {
			case mf.GetType() == dto.MetricType_SUMMARY:
				metrics.Gauge(metricName+"_count", float64(m.GetSummary().GetSampleCount()))
				metrics.Gauge(metricName+"_sum", m.GetSummary().GetSampleSum())
				for qn, qv := range getQuantiles(m) {
					metrics.GaugeWithTags(metricName+"_"+qn, tags, qv)
				}
			case mf.GetType() == dto.MetricType_HISTOGRAM:
				metrics.Gauge(metricName+"_count", float64(m.GetHistogram().GetSampleCount()))
				metrics.Gauge(metricName+"_sum", m.GetHistogram().GetSampleSum())
				for bn, bv := range getBuckets(m) {
					metrics.GaugeWithTags(metricName+"_"+bn, tags, bv)
				}
			default:
				switch {
				case m.GetGauge() != nil:
					if m.GetGauge().Value != nil { //nolint:protogetter
						metrics.GaugeWithTags(metricName, tags, *m.GetGauge().Value) //nolint:protogetter
					}
				case m.GetCounter() != nil:
					if m.GetCounter().Value != nil { //nolint:protogetter
						metrics.GaugeWithTags(metricName, tags, *m.GetCounter().Value) //nolint:protogetter
					}
				case m.GetUntyped() != nil:
					if m.GetUntyped().Value != nil { //nolint:protogetter
						if *m.GetUntyped().Value == math.Inf(+1) { //nolint:protogetter
							logger.Warn().Str("metric", metricName).Str("type", mf.GetType().String()).Str("value", (*m).GetUntyped().String()).Msg("cannot coerce +Inf to uint64")
							continue
						}
						metrics.GaugeWithTags(metricName, tags, *m.GetUntyped().Value) //nolint:protogetter
					}
				}
			}
		}
	}

	return nil
}

func getLabels(m *dto.Metric) tags.Tags {
	labels := make([]string, 0, len(m.GetLabel()))
	for _, label := range m.GetLabel() {
		if label.GetName() != "" && label.GetValue() != "" {
			ln := nameCleanerRx.ReplaceAllString(label.GetName(), "")
			lv := nameCleanerRx.ReplaceAllString(label.GetValue(), "")
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

	return tags.FromList(baseTags)
}

func getQuantiles(m *dto.Metric) map[string]float64 {
	ret := make(map[string]float64)
	for _, q := range m.GetSummary().GetQuantile() {
		if q.Value != nil && !math.IsNaN(*q.Value) { //nolint:protogetter
			ret[fmt.Sprint(*q.Quantile)] = *q.Value //nolint:protogetter
		}
	}
	return ret
}

func getBuckets(m *dto.Metric) map[string]uint64 {
	ret := make(map[string]uint64)
	for _, b := range m.GetHistogram().GetBucket() {
		if b.CumulativeCount != nil { //nolint:protogetter
			ret[fmt.Sprint(*b.UpperBound)] = *b.CumulativeCount //nolint:protogetter
		}
	}
	return ret
}
