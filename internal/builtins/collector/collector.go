// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package collector

import (
	"errors"

	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/rs/zerolog"
)

// Collector defines the interface for builtin metric collectors
type Collector interface {
	Collect() error
	Flush() cgm.Metrics
	ID() string
	Inventory() InventoryStats
	Logger() zerolog.Logger
}

// InventoryStats defines the stats a collector exposes for the /inventory endpoint
type InventoryStats struct {
	ID              string `json:"name"`
	LastError       string `json:"last_error"`
	LastRunDuration string `json:"last_run_duration"`
	LastRunEnd      string `json:"last_run_end"`
	LastRunStart    string `json:"last_run_start"`
}

var (
	// ErrNotImplemented collector type is not implemented on this os
	ErrNotImplemented = errors.New("Not implemented on OS")

	// ErrAlreadyRunning collector is already running
	ErrAlreadyRunning = errors.New("Already running")

	// ErrTTLNotExpired collector run ttl has not expired
	ErrTTLNotExpired = errors.New("TTL not expired")
)
