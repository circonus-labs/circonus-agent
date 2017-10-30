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

	tomb "gopkg.in/tomb.v2"

	"github.com/circonus-labs/circonus-agent/internal/builtins"
	"github.com/circonus-labs/circonus-agent/internal/plugins"
	"github.com/circonus-labs/circonus-agent/internal/statsd"
	"github.com/rs/zerolog"
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
	metrics map[string]interface{}
	ts      time.Time
}

var (
	pluginPathRx    = regexp.MustCompile("^/(run(/.*)?)?$")
	inventoryPathRx = regexp.MustCompile("^/inventory/?$")
	writePathRx     = regexp.MustCompile("^/write/.+$")
	statsPathRx     = regexp.MustCompile("^/stats/?$")
	promPathRx      = regexp.MustCompile("^/prom/?$")
	lastMetrics     = &previousMetrics{}
	lastMeticsmu    sync.Mutex
)
