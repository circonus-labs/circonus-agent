// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package statsd

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/maier/go-appstats"
	"github.com/spf13/viper"
)

// processPacket parses a packet for metrics.
func (s *Server) processPacket(pkt []byte) {
	if len(pkt) == 0 {
		return
	}

	s.logger.Debug().Str("packet", string(pkt)).Msg("received")
	metrics := bytes.Split(pkt, []byte("\n"))
	for _, metric := range metrics {
		if err := s.parseMetric(string(metric)); err != nil {
			_ = appstats.IncrementInt("statsd_metrics_bad")
			s.logger.Warn().Err(err).Str("metric", string(metric)).Msg("parsing")
		}
	}
}

// getMetricDestination determines "where" a metric should be sent (host or group)
// and cleans up the metric name if it matches a host|group prefix.
func (s *Server) getMetricDestination(metricName string) (string, string) {
	if s.hostPrefix == "" && s.groupPrefix == "" { // no host/group prefixes - send all metrics to host
		return destHost, metricName
	}

	if s.hostPrefix != "" && s.groupPrefix != "" { // explicit host and group, otherwise ignore
		if strings.HasPrefix(metricName, s.hostPrefix) {
			return destHost, strings.Replace(metricName, s.hostPrefix, "", 1)
		}
		if strings.HasPrefix(metricName, s.groupPrefix) {
			return destGroup, strings.Replace(metricName, s.groupPrefix, "", 1)
		}
		s.logger.Debug().Str("metric_name", metricName).Msg("does not match host|group prefix, ignoring")
		return destIgnore, metricName
	}

	if s.groupPrefix != "" && s.hostPrefix == "" { // default to host
		if strings.HasPrefix(metricName, s.groupPrefix) {
			return destGroup, strings.Replace(metricName, s.groupPrefix, "", 1)
		}
		return destHost, metricName
	}

	if s.groupPrefix == "" && s.hostPrefix != "" { // default to group
		if strings.HasPrefix(metricName, s.hostPrefix) {
			return destHost, strings.Replace(metricName, s.hostPrefix, "", 1)
		}
		return destGroup, metricName
	}

	s.logger.Debug().Str("metric_name", metricName).Msg("does not match host|group criteria, ignoring")
	return destIgnore, metricName
}

func (s *Server) parseMetric(metric string) error {
	// ignore 'blank' lines/empty strings
	if len(metric) == 0 {
		return nil
	}

	metricName := ""
	metricType := ""
	metricValue := ""
	metricRate := ""
	sampleRate := 1.0
	metricTagSpec := ""

	if !s.metricRegex.MatchString(metric) {
		return fmt.Errorf("invalid metric format '%s', ignoring", metric) //nolint:goerr113
	}

	for _, match := range s.metricRegex.FindAllStringSubmatch(metric, -1) {
		for gIdx, matchVal := range match {
			switch s.metricRegexGroupNames[gIdx] {
			case "name":
				metricName = s.nameSpaceReplaceRx.ReplaceAllString(matchVal, "_")
			case "type":
				metricType = matchVal
			case "value":
				metricValue = matchVal
			case "sample":
				metricRate = matchVal
			case "tags":
				metricTagSpec = matchVal
			default:
				// ignore any other groups
			}
		}
	}

	if metricName == "" || metricValue == "" {
		return fmt.Errorf("empty metric name (%s) or metric value (%s) - metricRegex failed, check", metricName, metricValue) //nolint:goerr113
	}

	if metricRate != "" {
		r, err := strconv.ParseFloat(metricRate, 32)
		if err != nil {
			return fmt.Errorf("invalid metric sampling rate: %w, ignoring", err)
		}
		sampleRate = r
		if sampleRate == 0.0 {
			return fmt.Errorf("invalid sample rate %f", sampleRate) //nolint:goerr113
		}
	}

	var (
		dest       *cgm.CirconusMetrics
		metricDest string
	)
	metricDest, metricName = s.getMetricDestination(metricName)

	if metricDest == destGroup {
		dest = s.groupMetrics
	} else if metricDest == destHost {
		dest = s.hostMetrics
		// metricName = s.hostCategory + config.MetricNameSeparator + metricName
	}

	if dest == nil {
		return fmt.Errorf("invalid metric destination (%s)->(%s)", metric, metricDest) //nolint:goerr113
	}

	// add stream tags to metric name
	metricTagList := []string{}
	if metricTagSpec != "" {
		metricTagList = strings.Split(metricTagSpec, tags.Separator)
	}
	tagList := make([]string, 0, len(s.baseTags)+len(metricTagList))
	tagList = append(tagList, s.baseTags...)
	tagList = append(tagList, metricTagList...)
	metricTags := tags.FromList(tagList)

	if s.debugMetricParsing {
		s.logger.Debug().
			Str("destination", metricDest).
			Str("metric", metric).
			Str("name", metricName).
			Str("type", metricType).
			Str("value", metricValue).
			Float64("rate", sampleRate).
			Strs("mtags", metricTagList).
			Msg("parsed")
	}

	switch metricType {
	case "c": // counter
		v, err := strconv.ParseUint(metricValue, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid counter value: %w", err)
		}
		val := int64(math.Round(float64(v) / sampleRate))
		metricTags = append(metricTags, cgm.Tag{Category: "statsd_type", Value: "count"})
		// counters always go in bin 0
		dest.RecordCountForValueWithTags(metricName, metricTags, 0, val)
		// s.logger.Debug().
		// 	Str("metric_name", metricName).
		// 	Float64("sample_rate", sampleRate).
		// 	Float64("bin", 0).
		// 	Int64("val", val).
		// 	Msg("RecordCountForValueWithTags - COUNTER")

	case "g": // gauge
		var val interface{}
		switch {
		case strings.Contains(metricValue, "."):
			v, err := strconv.ParseFloat(metricValue, 64)
			if err != nil {
				return fmt.Errorf("invalid gauge value: %w", err)
			}
			val = v
		case strings.Contains(metricValue, "-"):
			v, err := strconv.ParseInt(metricValue, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid gauge value: %w", err)
			}
			val = v
		default:
			v, err := strconv.ParseUint(metricValue, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid gauge value: %w", err)
			}
			val = v
		}
		if viper.GetBool(config.KeyClusterEnabled) && viper.GetBool(config.KeyClusterStatsdHistogramGauges) {
			dest.RemoveHistogramWithTags(metricName, metricTags)
			dest.SetHistogramValueWithTags(metricName, metricTags, val.(float64))
		} else {
			metricTags = append(metricTags, cgm.Tag{Category: "statsd_type", Value: "gauge"})
			dest.GaugeWithTags(metricName, metricTags, val)
		}
	case "h": // histogram (circonus)
		v, err := strconv.ParseFloat(metricValue, 64)
		if err != nil {
			return fmt.Errorf("invalid histogram value: %w", err)
		}
		val := int64(math.Round(1.0 / sampleRate))
		dest.RecordCountForValueWithTags(metricName, metricTags, v, val)
	case "ms": // timing measurement
		v, err := strconv.ParseFloat(metricValue, 64)
		if err != nil {
			return fmt.Errorf("invalid histogram value: %w", err)
		}
		metricTags = append(metricTags, cgm.Tag{Category: "statsd_type", Value: "timing"})
		val := int64(math.Round(1.0 / sampleRate))
		dest.RecordCountForValueWithTags(metricName, metricTags, v, val)
		// s.logger.Debug().
		// 	Str("metric_name", metricName).
		// 	Float64("sample_rate", sampleRate).
		// 	Float64("bin", v).
		// 	Int64("val", val).
		// 	Msg("RecordCountForValueWithTags - TIMING")
	case "s": // set
		// in the case of sets, the value is the unique "thing" to be tracked
		// counters are used to track individual "things"
		metricTags = append(metricTags, cgm.Tags{
			cgm.Tag{Category: "set_id", Value: metricValue},
			cgm.Tag{Category: "statsd_type", Value: "count"},
		}...)
		dest.RecordCountForValueWithTags(metricName, metricTags, 0, 1)
	case "t": // text (circonus)
		dest.SetTextWithTags(metricName, metricTags, metricValue)
	default:
		return fmt.Errorf("invalid metric type (%s)", metricType) //nolint:goerr113
	}

	return nil
}
