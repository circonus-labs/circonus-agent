// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package statsd

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/maier/go-appstats"
	"github.com/pkg/errors"
)

// processPacket parses a packet for metrics
func (s *Server) processPacket(pkt []byte) error {
	if len(pkt) == 0 {
		return nil
	}

	s.logger.Debug().Str("packet", string(pkt)).Msg("received")
	metrics := bytes.Split(pkt, []byte("\n"))
	for _, metric := range metrics {
		if err := s.parseMetric(string(metric)); err != nil {
			_ = appstats.IncrementInt("statsd_metrics_bad")
			s.logger.Warn().Err(err).Str("metric", string(metric)).Msg("parsing")
		}
	}

	return nil
}

// getMetricDestination determines "where" a metric should be sent (host or group)
// and cleans up the metric name if it matches a host|group prefix
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
	sampleRate := 0.0
	metricTagSpec := ""

	if !s.metricRegex.MatchString(metric) {
		return errors.Errorf("invalid metric format '%s', ignoring", metric)
	}

	for _, match := range s.metricRegex.FindAllStringSubmatch(metric, -1) {
		for gIdx, matchVal := range match {
			switch s.metricRegexGroupNames[gIdx] {
			case "name":
				metricName = matchVal
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
		return errors.Errorf("empty metric name (%s) or metric value (%s) - metricRegex failed, check", metricName, metricValue)
	}

	if metricRate != "" {
		r, err := strconv.ParseFloat(metricRate, 32)
		if err != nil {
			return errors.Errorf("invalid metric sampling rate (%s), ignoring", err)
		}
		sampleRate = r
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
		return errors.Errorf("invalid metric destination (%s)->(%s)", metric, metricDest)
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

	switch metricType {
	case "c": // counter
		v, err := strconv.ParseUint(metricValue, 10, 64)
		if err != nil {
			return errors.Wrap(err, "invalid counter value")
		}
		if sampleRate > 0 {
			v = uint64(float64(v) * (1 / sampleRate))
		}
		dest.IncrementByValueWithTags(metricName, metricTags, v)
	case "g": // gauge
		if strings.Contains(metricValue, ".") {
			v, err := strconv.ParseFloat(metricValue, 64)
			if err != nil {
				return errors.Wrap(err, "invalid gauge value")
			}
			dest.GaugeWithTags(metricName, metricTags, v)
		} else if strings.Contains(metricValue, "-") {
			v, err := strconv.ParseInt(metricValue, 10, 64)
			if err != nil {
				return errors.Wrap(err, "invalid gauge value")
			}
			dest.GaugeWithTags(metricName, metricTags, v)
		} else {
			v, err := strconv.ParseUint(metricValue, 10, 64)
			if err != nil {
				return errors.Wrap(err, "invalid gauge value")
			}
			dest.GaugeWithTags(metricName, metricTags, v)
		}
	case "h": // histogram (circonus)
		fallthrough
	case "ms": // measurement
		v, err := strconv.ParseFloat(metricValue, 64)
		if err != nil {
			return errors.Wrap(err, "invalid histogram value")
		}
		if sampleRate > 0 {
			v /= sampleRate
		}
		dest.RecordValueWithTags(metricName, metricTags, v)
	case "s": // set
		// in the case of sets, the value is the unique "thing" to be tracked
		// counters are used to track individual "things"
		metricTags = append(metricTags, cgm.Tag{Category: "set_id", Value: metricValue})
		dest.IncrementWithTags(metricName, metricTags)
	case "t": // text (circonus)
		dest.SetTextWithTags(metricName, metricTags, metricValue)
	default:
		return errors.Errorf("invalid metric type (%s)", metricType)
	}

	s.logger.Debug().
		Str("destination", metricDest).
		Str("metric", metric).
		Str("name", metricName).
		Str("type", metricType).
		Str("value", metricValue).
		Msg("parsing")

	return nil
}
