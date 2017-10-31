// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package agent

import (
	"os"

	tomb "gopkg.in/tomb.v2"

	"github.com/circonus-labs/circonus-agent/internal/builtins"
	"github.com/circonus-labs/circonus-agent/internal/plugins"
	"github.com/circonus-labs/circonus-agent/internal/reverse"
	"github.com/circonus-labs/circonus-agent/internal/server"
	"github.com/circonus-labs/circonus-agent/internal/statsd"
)

// Agent holds the main circonus-agent process
type Agent struct {
	builtins     *builtins.Builtins
	listenServer *server.Server
	plugins      *plugins.Plugins
	reverseConn  *reverse.Connection
	signalCh     chan os.Signal
	statsdServer *statsd.Server
	t            tomb.Tomb
}
