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

	"github.com/rs/zerolog"
)

// Metric defines an individual metric sample or array of samples (histogram)
type Metric struct {
	Type  string      `json:"_type"`
	Value interface{} `json:"_value"`
}

// Metrics defines the list of metrics for a given plugin
type Metrics map[string]Metric

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
	metrics         *Metrics
	Name            string
	prevMetrics     *Metrics
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
