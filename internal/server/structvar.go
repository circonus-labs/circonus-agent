// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"context"
	"net/http"
	"regexp"

	"github.com/circonus-labs/circonus-agent/internal/plugins"
	"github.com/circonus-labs/circonus-agent/internal/statsd"
	"github.com/rs/zerolog"
)

// Server defines the listening servers
type Server struct {
	ctx       context.Context
	logger    zerolog.Logger
	plugins   *plugins.Plugins
	svrHTTP   *http.Server
	svrHTTPS  *http.Server
	statsdSvr *statsd.Server
}

var (
	pluginPathRx    = regexp.MustCompile("^/(run(/.*)?)?$")
	inventoryPathRx = regexp.MustCompile("^/inventory/?$")
	writePathRx     = regexp.MustCompile("^/write/.+$")
)
