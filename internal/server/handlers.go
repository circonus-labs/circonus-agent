// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/plugins"
	"github.com/circonus-labs/circonus-agent/internal/server/receiver"
	cgm "github.com/circonus-labs/circonus-gometrics"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// run handles requests to execute plugins and return metrics emitted
// handles /, /run, or /run/plugin_name
func (s *Server) run(w http.ResponseWriter, r *http.Request) {
	plugin := ""

	if strings.HasPrefix(r.URL.Path, "/run/") { // run specific plugin
		plugin = strings.Replace(r.URL.Path, "/run/", "", -1)
		if plugin != "" {
			if !s.plugins.IsInternal(plugin) && !s.plugins.IsValid(plugin) {
				s.logger.Warn().
					Str("plugin", plugin).
					Msg("Unknown plugin requested")
				http.NotFound(w, r)
				return
			}
		}
	}

	lastMeticsmu.Lock()
	defer lastMeticsmu.Unlock()

	metrics := &map[string]interface{}{}

	if plugin == "" || !s.plugins.IsInternal(plugin) {
		// NOTE: errors are ignored from plugins.Run
		//       1. errors are already logged by Run
		//       2. do not expose execution state to callers
		s.plugins.Run(plugin)
		metrics = s.plugins.Flush(plugin)
	}

	if plugin == "" || plugin == "write" {
		receiverMetrics := receiver.Flush()
		for metricGroup, value := range *receiverMetrics {
			(*metrics)[metricGroup] = value
		}
	}

	if plugin == "" || plugin == "statsd" {
		if s.statsdSvr != nil {
			statsdMetrics := s.statsdSvr.Flush()
			if statsdMetrics != nil {
				(*metrics)[viper.GetString(config.KeyStatsdHostCategory)] = *statsdMetrics
			}
		}
	}

	lastMetrics.metrics = metrics
	lastMetrics.ts = time.Now()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		s.logger.Error().
			Err(err).
			Msg("Writing metrics to response")
	}
}

// inventory returns the current, active plugin inventory
func (s *Server) inventory(w http.ResponseWriter, r *http.Request) {
	inventory := s.plugins.Inventory()
	if inventory == nil {
		inventory = []byte(`{"error": "empty inventory"}`)
		s.logger.Error().Msg("inventory is nil/empty...")
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(inventory)
}

// write handles PUT/POST requests with a JSON playload containing "freeform"
// metrics. No validation is applied to the "format" of the metrics beyond k/v.
// Where 'key' is the metric name and 'value' is the metric value as either a
// simple value (e.g. {"name": 1, "foo": "bar", ...}) or a structured value
// representation (e.g. {"foo": {_type: "i", _value: 1}, ...}).
func (s *Server) write(w http.ResponseWriter, r *http.Request) {
	id := strings.Replace(r.URL.Path, "/write/", "", -1)

	log.Debug().Str("path", r.URL.Path).Str("id", id).Msg("write request")
	// a write request *MUST* include a metric group id to act as a namespace.
	// in other words, a "plugin name", all metrics for that write will appear
	// _under_ the metric group id (aka plugin name)
	if id == "" {
		http.NotFound(w, r)
		return
	}

	if err := receiver.Parse(id, r.Body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// promOutput returns the last metrics in prom format
func (s *Server) promOutput(w http.ResponseWriter, r *http.Request) {
	if lastMetrics.metrics == nil || len(*lastMetrics.metrics) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	ms := lastMetrics.ts.UnixNano() / int64(time.Millisecond)

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	for group, data := range *lastMetrics.metrics {
		metricsToPromFormat(w, group, ms, data)
	}
}

func metricsToPromFormat(w io.Writer, prefix string, ts int64, val interface{}) {
	switch t := val.(type) {
	case cgm.Metric:
		metric := val.(cgm.Metric)
		s := fmt.Sprintf("%v", metric.Value)
		switch metric.Type {
		case "i":
			fallthrough
		case "I":
			fallthrough
		case "l":
			fallthrough
		case "L":
			v, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				log.Error().Err(err).Msg("conv int64")
				return
			}
			if _, err := w.Write([]byte(fmt.Sprintf("%s %d %d\n", prefix, v, ts))); err != nil {
				log.Error().Err(err).Msg("writing prom output")
			}
		case "n":
			if strings.Contains(s, "[H[") {
				if _, err := w.Write([]byte(fmt.Sprintf("#HISTOGRAM %s %s %d\n", prefix, s, ts))); err != nil {
					log.Error().Err(err).Msg("writing prom output")
				}
			} else {
				v, err := strconv.ParseFloat(s, 64)
				if err != nil {
					log.Error().Err(err).Msg("conv float64")
					return
				}
				if _, err := w.Write([]byte(fmt.Sprintf("%s %f %d\n", prefix, v, ts))); err != nil {
					log.Error().Err(err).Msg("writing prom output")
				}
			}
		default:
			if _, err := w.Write([]byte(fmt.Sprintf("#TEXT %s %s %d\n", prefix, s, ts))); err != nil {
				log.Error().Err(err).Msg("writing prom output")
			}
		}
	case cgm.Metrics:
		metrics, ok := val.(cgm.Metrics)
		if !ok {
			st := fmt.Sprintf("%T", t)
			log.Warn().Interface("val", val).Str("target_type", st).Str("pkg", "prom export").Msg("unable to coerce")
			return
		}
		for pfx, metric := range metrics {
			name := prefix
			if pfx != "" {
				name += "`" + pfx
			}
			metricsToPromFormat(w, name, ts, metric)
		}
	case *plugins.Metrics:
		metrics, ok := val.(*plugins.Metrics)
		if !ok {
			st := fmt.Sprintf("%T", t)
			log.Warn().Interface("val", val).Str("target_type", st).Str("pkg", "prom export").Msg("unable to coerce")
			return
		}
		for pfx, metric := range *metrics {
			name := prefix
			if pfx != "" {
				name += "`" + pfx
			}
			metricsToPromFormat(w, name, ts, cgm.Metric(metric))
		}
	default:
		if _, err := w.Write([]byte(fmt.Sprintf("#UNHANDLED TYPE(%T) %v = %#v\n", t, prefix, val))); err != nil {
			log.Error().Err(err).Msg("writing prom output")
		}
	}
}
