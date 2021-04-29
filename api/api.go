// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package api

import (
	"fmt"
	"net/url"
	"regexp"
)

// Client defines the circonus-agent api client configuration.
type Client struct {
	agentURL *url.URL
	pidVal   *regexp.Regexp
}

// Metric defines an individual metric.
type Metric struct {
	Value interface{} `json:"_value"`
	Type  string      `json:"_type"`
}

// Metrics holds host metrics.
type Metrics map[string]Metric

// Inventory defines list of active plugins.
type Inventory []Plugin

// Plugin defines an active plugin.
type Plugin struct {
	ID              string   `json:"id"` // combination of name`instance
	Name            string   `json:"name"`
	Instance        string   `json:"instance"`
	Command         string   `json:"command"`
	LastRunStart    string   `json:"last_run_start"`
	LastRunEnd      string   `json:"last_run_end"`
	LastRunDuration string   `json:"last_run_duration"`
	LastError       string   `json:"last_error"`
	Args            []string `json:"args"`
}

var (
	errInvalidAgentURL     = fmt.Errorf("invalid agent URL (empty)")
	errInvalidRequestPath  = fmt.Errorf("invalid request path (empty)")
	errInvalidHTTPResponse = fmt.Errorf("invalid HTTP response")
	errInvalidPluginID     = fmt.Errorf("invalid plugin ID")
	errInvalidGroupID      = fmt.Errorf("invalid group id (empty)")
	errInvalidMetrics      = fmt.Errorf("invalid metrics (nil)")
	errInvalidMetricList   = fmt.Errorf("invalid metrics (none)")
)

// New creates a new circonus-agent api client.
func New(agentURL string) (*Client, error) {
	if agentURL == "" {
		return nil, errInvalidAgentURL
	}

	u, err := url.Parse(agentURL)
	if err != nil {
		return nil, fmt.Errorf("url parse: %w", err)

	}
	pv := regexp.MustCompile("^[a-zA-Z0-9_-`]+$") // e.g. pluginName or pluginName`instanceID

	return &Client{agentURL: u, pidVal: pv}, nil
}
