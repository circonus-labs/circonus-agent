// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"expvar"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/maier/go-appstats"
	"github.com/spf13/viper"
)

func (s *Server) router(w http.ResponseWriter, r *http.Request) {
	_ = appstats.IncrementInt("server.requests_total")

	s.logger.Debug().Str("method", r.Method).Str("url", r.URL.String()).Msg("request")

	switch r.Method {
	case "GET":
		switch {
		case r.URL.Path == "/health", r.URL.Path == "/health/":
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, "Alive")
		case pluginPathRx.MatchString(r.URL.Path): // run plugin(s)
			if viper.GetBool(config.KeyMultiAgent) {
				http.Error(w, "not allowed when multi-agent enabled", http.StatusForbidden)
				return
			}
			if err := appstats.SetString("server.last_run_request", time.Now().String()); err != nil {
				s.logger.Warn().Err(err).Msg("setting app stat - last_run_request")
			}
			s.run(w, r)
		case inventoryPathRx.MatchString(r.URL.Path): // plugin inventory
			s.inventory(w)
		case statsPathRx.MatchString(r.URL.Path): // app stats
			expvar.Handler().ServeHTTP(w, r)
		case promPathRx.MatchString(r.URL.Path): // output prom format...
			s.promOutput(w)
		case strings.HasPrefix(r.URL.Path, "/options"):
			s.handleOptions(w, r)
		default:
			_ = appstats.IncrementInt("server.requests_bad")
			s.logger.Warn().Str("method", r.Method).Str("url", r.URL.String()).Msg("not found")
			http.NotFound(w, r)
		}
	case "POST":
		fallthrough
	case "PUT":
		switch {
		case writePathRx.MatchString(r.URL.Path):
			s.write(w, r)
		case promPathRx.MatchString(r.URL.Path):
			s.promReceiver(w, r)
		default:
			_ = appstats.IncrementInt("server.requests_bad")
			s.logger.Warn().Str("method", r.Method).Str("url", r.URL.String()).Msg("not found")
			http.NotFound(w, r)
		}
	default:
		_ = appstats.IncrementInt("server.requests_bad")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
