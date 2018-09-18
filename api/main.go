// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package api

import (
	"net/url"
	"regexp"

	"github.com/pkg/errors"
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

// New creates a new circonus-agent api client
func New(agentURL string) (*Client, error) {
	if agentURL == "" {
		return nil, errors.New("invalid agent URL (empty)")
	}

	u, err := url.Parse(agentURL)
	if err != nil {
		return nil, err
	}
	pv, err := regexp.Compile("^[a-zA-Z0-9_-`]+$") // e.g. pluginName or pluginName`instanceID
	if err != nil {
		return nil, errors.Wrap(err, "plugin name validator")
	}

	return &Client{agentURL: u, pidVal: pv}, nil
}
