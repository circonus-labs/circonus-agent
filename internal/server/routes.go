// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"net/http"
	"regexp"
)

var (
	pluginPathRx    = regexp.MustCompile("^/(run(/.*)?)?$")
	inventoryPathRx = regexp.MustCompile("^/inventory/?$")
	writePathRx     = regexp.MustCompile("^/write/.+$")
)

func router(w http.ResponseWriter, r *http.Request) {

	logger.Info().
		Str("method", r.Method).
		Str("url", r.URL.String()).
		Msg("Request")

	switch r.Method {
	case "GET":
		if pluginPathRx.MatchString(r.URL.Path) { // run plugin(s)
			run(w, r)
		} else if inventoryPathRx.MatchString(r.URL.Path) { // plugin inventory
			inventory(w, r)
		} else {
			logger.Warn().
				Str("method", r.Method).
				Str("url", r.URL.String()).
				Msg("Not found")
			http.NotFound(w, r)
		}
	case "POST":
		fallthrough
	case "PUT":
		if writePathRx.MatchString(r.URL.Path) {
			write(w, r)
		} else {
			logger.Warn().
				Str("method", r.Method).
				Str("url", r.URL.String()).
				Msg("Not found")
			http.NotFound(w, r)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

}
