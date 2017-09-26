// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package agent

import (
	"context"
	"os"

	"github.com/circonus-labs/circonus-agent/internal/plugins"
	"github.com/circonus-labs/circonus-agent/internal/reverse"
	"github.com/circonus-labs/circonus-agent/internal/server"
	"github.com/circonus-labs/circonus-agent/internal/statsd"
)

// Agent holds the main circonus-agent process
type Agent struct {
	errCh        chan error
	listenServer *server.Server
	plugins      *plugins.Plugins
	reverseConn  *reverse.Connection
	shutdown     func()
	shutdownCtx  context.Context
	signalCh     chan os.Signal
	statsdServer *statsd.Server
}
