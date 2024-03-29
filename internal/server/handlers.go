// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/circonus-labs/circonus-agent/internal/server/promrecv"
	"github.com/circonus-labs/circonus-agent/internal/server/receiver"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	appstats "github.com/maier/go-appstats"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

const (
	conduitBuiltin    = "builtins"
	conduitPlugin     = "plugins"
	conduitReceiver   = "receiver"
	conduitStatsd     = "statsd"
	conduitPrometheus = "prometheus"
)

// run handles requests to execute plugins and return metrics emitted
// handles /, /run, or /run/plugin_name.
func (s *Server) run(w http.ResponseWriter, r *http.Request) {
	runStart := time.Now()
	id := ""

	if strings.HasPrefix(r.URL.Path, "/run/") { // run specific item
		id = strings.ReplaceAll(r.URL.Path, "/run/", "")
		if id != "" {
			idOK := false

			// highest priority, internal servers (receiver, statsd, etc.)
			if !idOK {
				s.logger.Debug().Str("id", id).Msg("checking internals")
				idOK = s.plugins.IsInternal(id)
			}
			// check builtins before plugins, builtins offer better efficiency
			if !idOK {
				s.logger.Debug().Str("id", id).Msg("checking bulitins")
				idOK = s.builtins.IsBuiltin(id)
			}
			// lastly, check active plugins, if any
			if !idOK {
				s.logger.Debug().Str("id", id).Msg("checking plugins")
				idOK = s.plugins.IsValid(id)
			}

			if !idOK {
				s.logger.Warn().
					Str("id", id).
					Msg("unknown item requested")
				http.NotFound(w, r)
				return
			}
		}
	}

	conduitList := []string{} // default, empty list defaults to all known
	if id != "" {
		// identify conduit to collect from based on id passed
		switch {
		case id == conduitPrometheus || id == "prom":
			conduitList = []string{conduitPrometheus}
		case id == conduitReceiver || id == "write":
			conduitList = []string{conduitReceiver}
		case id == conduitStatsd:
			conduitList = []string{conduitStatsd}
		case s.builtins.IsBuiltin(id):
			conduitList = []string{conduitBuiltin}
		default:
			conduitList = []string{conduitPlugin}
		}
	}

	metrics := s.GetMetrics(conduitList, id)
	s.logger.Debug().Int("num_metrics", len(metrics)).Msg("aggregated")

	lastMetricsmu.Lock()
	lastMetrics.metrics = &metrics
	lastMetrics.ts = time.Now()
	lastMetricsmu.Unlock()

	// if err := s.check.EnableNewMetrics(&metrics); err != nil {
	// 	s.logger.Warn().Err(err).Msg("unable to update check bundle metrics")
	// }

	s.encodeResponse(&metrics, w, r, runStart)
}

// GetMetrics collects metrics from the various conduits and returns them for disposition.
func (s *Server) GetMetrics(conduits []string, id string) cgm.Metrics {
	includeAgentMetrics := false
	// default to all conduits if list is empty
	if len(conduits) == 0 {
		conduits = []string{conduitBuiltin, conduitPlugin, conduitReceiver, conduitStatsd, conduitPrometheus}
		includeAgentMetrics = true
	}

	collectStart := time.Now()

	type conduit struct {
		metrics *cgm.Metrics
		id      string
	}
	conduitCh := make(chan conduit, len(conduits)) // number of conduits
	var wg sync.WaitGroup

	for _, cid := range conduits {
		switch cid {
		case conduitBuiltin:
			wg.Add(1)
			go func(conduitID string) {
				start := time.Now()
				numMetrics := 0
				s.logger.Debug().Str("conduit_id", conduitID).Msg("start")
				if err := s.builtins.Run(s.groupCtx, id); err != nil {
					s.logger.Error().Err(err).Str("id", id).Msg("running builtin")
				}
				builtinMetrics := s.builtins.Flush(id)
				if builtinMetrics != nil && len(*builtinMetrics) > 0 {
					numMetrics = len(*builtinMetrics)
					conduitCh <- conduit{id: conduitID, metrics: builtinMetrics}
				}
				s.logger.Debug().Str("conduit_id", conduitID).Str("duration", time.Since(start).String()).Int("metrics", numMetrics).Msg("done")
				wg.Done()
			}(cid)
		case conduitPlugin:
			wg.Add(1)
			go func(conduitID string) {
				// NOTE: errors are ignored from plugins.Run
				//       1. errors are already logged by Run
				//       2. do not expose execution state to callers
				start := time.Now()
				numMetrics := 0
				s.logger.Debug().Str("conduit_id", conduitID).Msg("start")
				if err := s.plugins.Run(id); err != nil {
					s.logger.Error().Err(err).Str("id", id).Msg("running plugin")
				}
				pluginMetrics := s.plugins.Flush(id)
				if pluginMetrics != nil && len(*pluginMetrics) > 0 {
					numMetrics = len(*pluginMetrics)
					conduitCh <- conduit{id: conduitID, metrics: pluginMetrics}
				}
				s.logger.Debug().Str("conduit_id", conduitID).Str("duration", time.Since(start).String()).Int("metrics", numMetrics).Msg("done")
				wg.Done()
			}(cid)
		case conduitReceiver:
			wg.Add(1)
			go func(conduitID string) {
				start := time.Now()
				numMetrics := 0
				s.logger.Debug().Str("conduit_id", conduitID).Msg("start")
				receiverMetrics := receiver.Flush()
				if receiverMetrics != nil && len(*receiverMetrics) > 0 {
					numMetrics = len(*receiverMetrics)
					conduitCh <- conduit{id: conduitID, metrics: receiverMetrics}
				}
				s.logger.Debug().Str("conduit_id", conduitID).Str("duration", time.Since(start).String()).Int("metrics", numMetrics).Msg("done")
				wg.Done()
			}(cid)
		case conduitStatsd:
			if s.statsdSvr != nil {
				wg.Add(1)
				go func(conduitID string) {
					start := time.Now()
					numMetrics := 0
					s.logger.Debug().Str("conduit_id", conduitID).Msg("start")
					statsdMetrics := s.statsdSvr.Flush()
					if statsdMetrics != nil && len(*statsdMetrics) > 0 {
						numMetrics = len(*statsdMetrics)
						conduitCh <- conduit{id: conduitID, metrics: statsdMetrics}
					}
					s.logger.Debug().Str("conduit_id", conduitID).Str("duration", time.Since(start).String()).Int("metrics", numMetrics).Msg("done")
					wg.Done()
				}(cid)
			}
		case conduitPrometheus:
			wg.Add(1)
			go func(conduitID string) {
				start := time.Now()
				numMetrics := 0
				s.logger.Debug().Str("conduit_id", conduitID).Msg("start")
				promMetrics := promrecv.Flush()
				if promMetrics != nil && len(*promMetrics) > 0 {
					numMetrics = len(*promMetrics)
					conduitCh <- conduit{id: conduitID, metrics: promMetrics}
				}
				s.logger.Debug().Str("conduit_id", conduitID).Str("duration", time.Since(start).String()).Int("metrics", numMetrics).Msg("done")
				wg.Done()
			}(cid)
		}
	}

	s.logger.Debug().Msg("waiting for metric collection from input conduits")
	wg.Wait()
	close(conduitCh)

	metrics := cgm.Metrics{}
	for cm := range conduitCh {
		for m, v := range *cm.metrics {
			metrics[m] = v
		}
	}

	cdur := time.Since(collectStart)

	if includeAgentMetrics {
		mtags := tags.GetBaseTags()
		mtags = append(mtags, []string{"collector:agent", "__rollup:false"}...)
		if viper.GetBool(config.KeyClusterEnabled) {
			if n := viper.GetString(config.KeyCheckTarget); n != "" {
				mtags = append(mtags, "node:"+n)
			}
		}
		metrics[tags.MetricNameWithStreamTags("agent_version", tags.FromList(mtags))] = cgm.Metric{Value: release.NAME + "_" + release.VERSION, Type: "s"}
		{
			var ctags []string
			ctags = append(ctags, mtags...)
			ctags = append(ctags, "units:milliseconds")
			metrics[tags.MetricNameWithStreamTags("agent_collect_duration", tags.FromList(ctags))] = cgm.Metric{Value: cdur.Milliseconds(), Type: "L"}
		}
		s.agentStats(metrics, mtags)
	}

	s.logger.Debug().Str("duration", cdur.String()).Msg("collection complete")

	return metrics
}

// encodeResponse takes care of encoding the response to an HTTP request for metrics.
// The broker does not handle chunk encoded data correctly and will emit an error if
// it receives it. The agent does support gzip compression when the correct header
// is supplied (Accept-Encoding: * or Accept-Encoding: gzip). The command line option
// --no-gzip overrides and will result in unencoded response regardless of what the
// Accept-Encoding header specifies.
func (s *Server) encodeResponse(m *cgm.Metrics, w http.ResponseWriter, r *http.Request, runStart time.Time) {
	//
	// if an error occurs, it is logged and empty {} metrics are returned
	//

	// basically, turn off chunking
	w.Header().Set("Transfer-Encoding", "identity")
	w.Header().Set("Content-Type", "application/json")

	var data []byte
	var jsonData []byte
	var err error
	var useGzip bool

	if viper.GetBool(config.KeyDisableGzip) {
		useGzip = false
	} else {
		acceptedEncodings := r.Header.Get("Accept-Encoding")
		useGzip = strings.Contains(acceptedEncodings, "*") || strings.Contains(acceptedEncodings, "gzip")
	}

	jsonData, err = json.Marshal(m)
	if err != nil {
		// log the error and respond with empty metrics
		s.logger.Error().
			Err(err).
			Interface("metrics", m).
			Msg("encoding metrics to JSON for response")
		jsonData = []byte("{}")
	}
	data = jsonData

	if useGzip {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err := gz.Write(jsonData)
		gz.Close()
		if err != nil {
			// log the error and respond with empty metrics
			s.logger.Error().
				Err(err).
				Msg("compressing metrics")
			data = []byte("{}")
		} else {
			w.Header().Set("Content-Encoding", "gzip")
			data = buf.Bytes()
		}
	}

	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	if _, err := w.Write(data); err != nil {
		s.logger.Error().
			Err(err).
			Msg("writing metrics to response")
		return
	}

	s.logger.Info().Str("duration", time.Since(runStart).String()).Int("num_metrics", len(*m)).Bool("compressed", useGzip).Int("content_bytes", len(data)).Msg("request response")

	dumpDir := viper.GetString(config.KeyDebugDumpMetrics)
	if dumpDir != "" {
		dumpFile := filepath.Join(dumpDir, "metrics_"+time.Now().Format("20060102_150405")+".json")
		if err := os.WriteFile(dumpFile, jsonData, 0644); err != nil { //nolint:gosec
			s.logger.Error().
				Err(err).
				Str("file", dumpFile).
				Msg("dumping metrics")
		}
	}
}

// inventory returns the current, active plugin inventory.
func (s *Server) inventory(w http.ResponseWriter) {
	inventory := s.plugins.Inventory()
	if inventory == nil {
		inventory = []byte(`{"error": "empty inventory"}`)
		s.logger.Error().Msg("inventory is nil/empty...")
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(inventory)
}

// socketHandler gates /write for the socket server only.
func (s *Server) socketHandler(w http.ResponseWriter, r *http.Request) {
	if !writePathRx.MatchString(r.URL.Path) {
		_ = appstats.IncrementInt("requests_bad")
		s.logger.Warn().
			Str("method", r.Method).
			Str("url", r.URL.String()).
			Msg("Not found")
		http.NotFound(w, r)
		return
	}

	if r.Method != "PUT" && r.Method != "POST" {
		_ = appstats.IncrementInt("requests_bad")
		s.logger.Warn().
			Str("method", r.Method).
			Str("url", r.URL.String()).
			Msg("Not found")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.write(w, r)
}

// write handles PUT/POST requests with a JSON playload containing "freeform"
// metrics. No validation is applied to the "format" of the metrics beyond k/v.
// Where 'key' is the metric name and 'value' is the metric value as either a
// simple value (e.g. {"name": 1, "foo": "bar", ...}) or a structured value
// representation (e.g. {"foo": {_type: "i", _value: 1}, ...}).
func (s *Server) write(w http.ResponseWriter, r *http.Request) {
	id := strings.ReplaceAll(r.URL.Path, "/write/", "")
	// a write request *MUST* include a metric group id to act as a namespace.
	// in other words, a "plugin name", all metrics for that write will appear
	// _under_ the metric group id (aka plugin name)
	if id == "" {
		s.logger.Warn().Msg("write recevier - invalid id (empty)")
		http.NotFound(w, r)
		return
	}

	if err := receiver.Parse(id, r.Body); err != nil {
		s.logger.Warn().Err(err).Msg("write recevier")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if meta, _ := s.check.CheckMeta(); meta != nil {
		// we ignore the error here intentionally; one of the modes
		// the agent can run in is for PULL, where it would have
		// no direct knowledge of what check bundle/check is pulling
		w.Header().Set("X-Circonus-Check-Bundle-ID", meta.BundleID)
		w.Header().Set("X-Circonus-Check-ID", meta.CheckID)
	}

	w.WriteHeader(http.StatusNoContent)
}

// promReceiver handles PUT/POST requests with prometheus TEXT formatted metrics
// https://prometheus.io/docs/instrumenting/exposition_formats/
func (s *Server) promReceiver(w http.ResponseWriter, r *http.Request) {
	s.logger.Debug().Str("path", r.URL.Path).Msg("prom metrics recevied")

	if err := promrecv.Parse(r.Body); err != nil {
		s.logger.Warn().Err(err).Msg("prom recevier")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if cm, _ := s.check.CheckMeta(); cm != nil {
		w.Header().Set("X-Circonus-Check-Bundle-ID", cm.BundleID)
		w.Header().Set("X-Circonus-Check-ID", cm.CheckID)
	}
	w.WriteHeader(http.StatusNoContent)
}

// promOutput returns the last metrics in prom format.
func (s *Server) promOutput(w http.ResponseWriter) {
	s.logger.Debug().Str("in", "prom output").Msg("start")

	s.logger.Debug().Str("in", "prom output").Msg("lock lastMetrics")
	lastMetricsmu.Lock()
	metrics := lastMetrics.metrics
	ms := lastMetrics.ts.UnixNano() / int64(time.Millisecond)
	lastMetricsmu.Unlock()
	s.logger.Debug().Str("in", "prom output").Msg("unlock lastMetrics")

	if metrics == nil || len(*metrics) == 0 {
		w.WriteHeader(http.StatusNoContent)
		s.logger.Debug().Str("in", "prom output").Msg("end")
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	for id, data := range *metrics {
		s.metricsToPromFormat(w, id, ms, data)
	}
	s.logger.Debug().Str("in", "prom output").Msg("end")
}

func (s *Server) metricsToPromFormat(w io.Writer, prefix string, ts int64, val interface{}) {
	l := s.logger.With().Str("op", "prom export").Logger()
	switch t := val.(type) {
	case cgm.Metric:
		metric := val.(cgm.Metric)
		sv := fmt.Sprintf("%v", metric.Value)
		switch metric.Type {
		case "i":
			fallthrough
		case "I":
			fallthrough
		case "l":
			fallthrough
		case "L":
			v, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				l.Error().Err(err).Msg("conv int64")
				return
			}
			if _, err := w.Write([]byte(fmt.Sprintf("%s %d %d\n", prefix, v, ts))); err != nil {
				l.Error().Err(err).Msg("writing prom output")
			}
		case "n":
			if strings.Contains(sv, "[H[") {
				l.Warn().
					Str("type", "histogram != [prom]histogram(percentile)").
					Str("metric", fmt.Sprintf("%s = %s", prefix, sv)).
					Msg("unsupported metric type")
			} else {
				v, err := strconv.ParseFloat(sv, 64)
				if err != nil {
					l.Error().Err(err).Msg("conv float64")
					return
				}
				if _, err := w.Write([]byte(fmt.Sprintf("%s %f %d\n", prefix, v, ts))); err != nil {
					l.Error().Err(err).Msg("writing prom output")
				}
			}
		case "s":
			l.Warn().
				Str("type", "text [prom]???").
				Str("metric", fmt.Sprintf("%s = %s", prefix, sv)).
				Msg("unsuported metric type")
		default:
			l.Warn().
				Str("type", metric.Type).
				Str("name", prefix).
				Interface("metric", metric).
				Msg("invalid metric type")
		}
	case cgm.Metrics:
		metrics := val.(cgm.Metrics)
		for pfx, metric := range metrics {
			name := prefix
			if pfx != "" {
				name = strings.Join([]string{name, pfx}, config.MetricNameSeparator)
			}
			s.metricsToPromFormat(w, name, ts, metric)
		}
	case *cgm.Metrics:
		metrics := val.(*cgm.Metrics)
		s.metricsToPromFormat(w, prefix, ts, *metrics)
	default:
		l.Warn().
			Str("metric", fmt.Sprintf("#TYPE(%T) %v = %#v", t, prefix, val)).
			Msg("unhandled export type")
	}
}

func (s *Server) handleOptions(w http.ResponseWriter, r *http.Request) {
	l := s.logger.With().Str("op", "options handler").Logger()
	values := r.URL.Query()
	for k, v := range values {
		switch strings.ToLower(k) {
		case "log_level":
			if len(v) > 0 {
				switch strings.ToLower(v[0]) {
				case "debug":
					zerolog.SetGlobalLevel(zerolog.DebugLevel)
					_, _ = w.Write([]byte("log level set to " + zerolog.DebugLevel.String()))
				case "info":
					zerolog.SetGlobalLevel(zerolog.InfoLevel)
					_, _ = w.Write([]byte("log level set to " + zerolog.InfoLevel.String()))
				case "warn":
					zerolog.SetGlobalLevel(zerolog.WarnLevel)
					_, _ = w.Write([]byte("log level set to " + zerolog.WarnLevel.String()))
				case "error":
					zerolog.SetGlobalLevel(zerolog.ErrorLevel)
					_, _ = w.Write([]byte("log level set to " + zerolog.ErrorLevel.String()))
				default:
					l.Warn().Str("level", v[0]).Msg("unknown log level")
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte("unknown log level " + v[0]))
				}
			}
		default:
			l.Warn().Str("arg", k).Msg("unknown option")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("unknown option " + k))
		}
	}
}
