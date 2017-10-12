// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package plugins

import (
	"context"
	"os/exec"
	"sync"
	"time"

	cgm "github.com/circonus-labs/circonus-gometrics"
	"github.com/rs/zerolog"
)

// Plugin defines a specific plugin
type plugin struct {
	cmd             *exec.Cmd
	Command         string
	ctx             context.Context
	Generation      uint64
	ID              string
	InstanceArgs    []string
	InstanceID      string
	LastError       error
	LastRunDuration time.Duration
	LastStart       time.Time
	logger          zerolog.Logger
	metrics         *cgm.Metrics
	Name            string
	prevMetrics     *cgm.Metrics
	RunDir          string
	Running         bool
	sync.Mutex
}

// Plugins defines plugin manager
type Plugins struct {
	active        map[string]*plugin
	ctx           context.Context
	generation    uint64
	logger        zerolog.Logger
	pluginDir     string
	reservedNames map[string]bool
	running       bool
	sync.RWMutex
}

const (
	fieldDelimiter  = "\t"
	metricDelimiter = "`"
	nullMetricValue = "[[null]]"
)
