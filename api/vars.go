// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package api

import (
	"net/url"
	"regexp"
)

// Client defines the circonus-agent api client configuration
type Client struct {
	agentURL *url.URL
	pidVal   *regexp.Regexp
}

// Metric defines an individual metric
type Metric struct {
	Type  string      `json:"_type"`
	Value interface{} `json:"_value"`
}

// Metrics holds host metrics
type Metrics map[string]Metric

// Inventory defines list of active plugins
type Inventory []Plugin

// Plugin defines an active plugin
type Plugin struct {
	ID              string   `json:"id"` // combination of name`instance
	Name            string   `json:"name"`
	Instance        string   `json:"instance"`
	Command         string   `json:"command"`
	Args            []string `json:"args"`
	LastRunStart    string   `json:"last_run_start"`
	LastRunEnd      string   `json:"last_run_end"`
	LastRunDuration string   `json:"last_run_duration"`
	LastError       string   `json:"last_error"`
}
