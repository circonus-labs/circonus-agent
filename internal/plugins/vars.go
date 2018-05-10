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

// Plugins defines plugin manager
type Plugins struct {
	active        map[string]*plugin
	ctx           context.Context
	logger        zerolog.Logger
	pluginDir     string
	reservedNames map[string]bool
	running       bool
	sync.RWMutex
}

// Plugin defines a specific plugin
type plugin struct {
	cmd             *exec.Cmd
	command         string
	ctx             context.Context
	id              string
	instanceArgs    []string
	instanceID      string
	lastError       error
	lastRunDuration time.Duration
	lastStart       time.Time
	lastEnd         time.Time
	logger          zerolog.Logger
	metrics         *cgm.Metrics
	name            string
	prevMetrics     *cgm.Metrics
	runDir          string
	running         bool
	runTTL          time.Duration
	sync.Mutex
}

// // pluginDetails are exposed via the /inventory endpoint
// type pluginDetails struct {
// 	Name            string   `json:"name"`
// 	Instance        string   `json:"instance"`
// 	Command         string   `json:"command"`
// 	Args            []string `json:"args"`
// 	LastRunStart    string   `json:"last_run_start"`
// 	LastRunEnd      string   `json:"last_run_end"`
// 	LastRunDuration string   `json:"last_run_duration"`
// 	LastError       string   `json:"last_error"`
// }

const (
	fieldDelimiter  = "\t"
	metricDelimiter = "`"
	nullMetricValue = "[[null]]"
)
