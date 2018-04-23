// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"context"
	"net"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins"
	"github.com/circonus-labs/circonus-agent/internal/check"
	"github.com/circonus-labs/circonus-agent/internal/plugins"
	"github.com/circonus-labs/circonus-agent/internal/statsd"
	cgm "github.com/circonus-labs/circonus-gometrics"
	"github.com/rs/zerolog"
	tomb "gopkg.in/tomb.v2"
)

type httpServer struct {
	address *net.TCPAddr
	server  *http.Server
}

type socketServer struct {
	address  *net.UnixAddr
	listener *net.UnixListener
	server   *http.Server
}

type sslServer struct {
	address  *net.TCPAddr
	certFile string
	keyFile  string
	server   *http.Server
}

// Server defines the listening servers
type Server struct {
	builtins   *builtins.Builtins
	check      *check.Check
	ctx        context.Context
	logger     zerolog.Logger
	plugins    *plugins.Plugins
	svrHTTP    []*httpServer
	svrHTTPS   *sslServer
	svrSockets []*socketServer
	statsdSvr  *statsd.Server
	t          tomb.Tomb
}

type previousMetrics struct {
	metrics cgm.Metrics
	ts      time.Time
}

var (
	pluginPathRx    = regexp.MustCompile("^/(run(/[a-zA-Z0-9_-]*)?)?$")
	inventoryPathRx = regexp.MustCompile("^/inventory/?$")
	writePathRx     = regexp.MustCompile("^/write/[a-zA-Z0-9_-]+$")
	statsPathRx     = regexp.MustCompile("^/stats/?$")
	promPathRx      = regexp.MustCompile("^/prom/?$")
	lastMetrics     = &previousMetrics{}
	lastMeticsmu    sync.Mutex
)
