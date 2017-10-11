// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/plugins"
	"github.com/circonus-labs/circonus-agent/internal/server/receiver"
	circonusgometrics "github.com/circonus-labs/circonus-gometrics"
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
	// s.logger.Debug().Interface("m", metrics).Msg("metrics")

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		s.logger.Error().
			Err(err).
			Msg("Writing metrics to response")
	}
}

// promOutput returns the last metrics in prom format
func (s *Server) promOutput(w http.ResponseWriter, r *http.Request) {
	if lastMetrics.metrics == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	ms := lastMetrics.ts.UnixNano() / int64(time.Millisecond)

	for group, data := range *lastMetrics.metrics {
		// t := reflect.TypeOf(data)
		// s.logger.Debug().Str("group", group).Str("type", t.String()).Msg("item")
		walkMetrics(w, group, ms, data)
	}
}

func walkMetrics(w http.ResponseWriter, prefix string, ts int64, val interface{}) {
	t := reflect.TypeOf(val)
	// log.Debug().Str("pfx", prefix).Str("type", t.String()).Interface("val", val).Msg("val")
	switch t.String() {
	case "circonusgometrics.Metrics":
		metrics, ok := val.(circonusgometrics.Metrics)
		if !ok {
			log.Warn().Interface("val", val).Str("target_type", t.String()).Str("pkg", "prom export").Msg("unable to coerce")
			return
		}
		for pfx, metric := range metrics {
			switch t2 := metric.Value.(type) {
			case uint64:
				w.Write([]byte(fmt.Sprintf("%s`%s %d %d\n", prefix, pfx, metric.Value, ts)))
			case float64:
				w.Write([]byte(fmt.Sprintf("%s`%s %f %d\n", prefix, pfx, metric.Value, ts)))
			case string:
				s := fmt.Sprintf("%v", metric.Value)
				ok, err := regexp.MatchString("^[0-9]+$", s)
				if err != nil {
					log.Error().Err(err).Msg("testing string for digits")
					continue
				}
				if ok {
					v, err := strconv.ParseInt(s, 10, 64)
					if err != nil {
						log.Error().Err(err).Msg("conv int64")
						continue
					}
					w.Write([]byte(fmt.Sprintf("%s`%s %d %d\n", prefix, pfx, v, ts)))
					continue
				}
				w.Write([]byte(fmt.Sprintf("#TEXT %s`%s %s %d\n", prefix, pfx, s, ts)))
			case []string:
				s := fmt.Sprintf("%v", metric.Value)
				if strings.Contains(s, "[H[") {
					w.Write([]byte(fmt.Sprintf("#HISTOGRAM %s`%s %s %d\n", prefix, pfx, s, ts)))
					continue
				}
				walkMetrics(w, prefix+"`"+pfx, ts, metric.Value)
			default:
				w.Write([]byte(fmt.Sprintf("?? %s`%s %v - %T\n", prefix, pfx, metric.Value, t2)))
				// walkMetrics(w, prefix+"`"+pfx, ts, metric.Value)
			}
		}
	case "*plugins.Metrics":
		for pfx, v := range *val.(*plugins.Metrics) {
			walkMetrics(w, prefix+"`"+pfx, ts, v.Value)
		}
	case "[]string":
		v, ok := val.([]string)
		if ok {
			if len(v) > 0 && v[0][0:2] == "H[" {
				w.Write([]byte(fmt.Sprintf("#HISTOGRAM %s %s %d\n", prefix, val, ts)))
				return
			}
			for idx, v2 := range v {
				w.Write([]byte(fmt.Sprintf("%s`%d %s %d\n", prefix, idx, v2, ts)))
			}
		}
	case "[]interface {}":
		v, ok := val.([]interface{})
		if !ok {
			log.Warn().Interface("val", val).Str("target_type", t.String()).Str("pkg", "prom export").Msg("unable to coerce")
			return
		}
		if fmt.Sprintf("%v", v)[0:3] == "[H[" {
			w.Write([]byte(fmt.Sprintf("#HISTOGRAM %s %s %d\n", prefix, val, ts)))
			return
		}
		for idx, v2 := range v {
			walkMetrics(w, fmt.Sprintf("%s`%d", prefix, idx), ts, v2)
		}
	case "map[string]interface {}":
		v, ok := val.(map[string]interface{})
		if !ok {
			log.Warn().Interface("val", val).Str("target_type", t.String()).Str("pkg", "prom export").Msg("unable to coerce")
			return
		}
		if _, isMetric := v["_type"]; isMetric {
			walkMetrics(w, prefix, ts, v["_value"])
		} else {
			for pfx, v2 := range v {
				walkMetrics(w, prefix+"`"+pfx, ts, v2)
			}
		}
	case "string":
		w.Write([]byte(fmt.Sprintf("#TEXT %s %s %d\n", prefix, val, ts)))
	case "float32":
		fallthrough
	case "float64":
		w.Write([]byte(fmt.Sprintf("%s %e %d\n", prefix, val, ts)))
	case "int32":
		fallthrough
	case "int64":
		fallthrough
	case "uint32":
		fallthrough
	case "uint64":
		w.Write([]byte(fmt.Sprintf("%s %d %d\n", prefix, val, ts)))
	default:
		w.Write([]byte(fmt.Sprintf("%v(%v) = %#v\n", prefix, t, val)))
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
