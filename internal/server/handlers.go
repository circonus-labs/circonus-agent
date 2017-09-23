// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/server/receiver"
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
