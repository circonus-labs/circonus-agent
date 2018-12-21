// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"expvar"
	"net/http"

	"github.com/maier/go-appstats"
)

func (s *Server) router(w http.ResponseWriter, r *http.Request) {
	appstats.IncrementInt("requests_total")

	s.logger.Debug().Str("method", r.Method).Str("url", r.URL.String()).Msg("request")

	switch r.Method {
	case "GET":
		if pluginPathRx.MatchString(r.URL.Path) { // run plugin(s)
			// s.logger.Debug().Msg("calling run")
			s.run(w, r)
			// s.logger.Debug().Msg("run complete")
		} else if inventoryPathRx.MatchString(r.URL.Path) { // plugin inventory
			s.inventory(w, r)
		} else if statsPathRx.MatchString(r.URL.Path) { // app stats
			expvar.Handler().ServeHTTP(w, r)
		} else if promPathRx.MatchString(r.URL.Path) { // output prom format...
			s.promOutput(w, r)
		} else {
			appstats.IncrementInt("requests_bad")
			s.logger.Warn().Str("method", r.Method).Str("url", r.URL.String()).Msg("not found")
			http.NotFound(w, r)
		}
	case "POST":
		fallthrough
	case "PUT":
		if writePathRx.MatchString(r.URL.Path) {
			s.write(w, r)
		} else if promPathRx.MatchString(r.URL.Path) {
			s.promReceiver(w, r)
		} else {
			appstats.IncrementInt("requests_bad")
			s.logger.Warn().Str("method", r.Method).Str("url", r.URL.String()).Msg("not found")
			http.NotFound(w, r)
		}
	default:
		appstats.IncrementInt("requests_bad")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
