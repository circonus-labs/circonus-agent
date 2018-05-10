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
