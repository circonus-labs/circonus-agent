// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package api

import (
	"context"
	"encoding/json"
	"fmt"
)

// Metrics retrieves metrics from one or all plugins
// NOTE: because the API is using the regular agent URL - the
//
//	agent will act as though any other client (e.g. a broker)
//	were requesting metrics - it will *run* the plugin(s).
func (c *Client) Metrics(pluginID string) (*Metrics, error) {
	return c.MetricsWithContext(context.Background(), pluginID)
}

// MetricsWithContext retrieves metrics from one or all plugins
// NOTE: because the API is using the regular agent URL - the
//
//	agent will act as though any other client (e.g. a broker)
//	were requesting metrics - it will *run* the plugin(s).
func (c *Client) MetricsWithContext(ctx context.Context, pluginID string) (*Metrics, error) {
	pid := ""
	if pluginID != "" {
		if !c.pidVal.MatchString(pluginID) {
			return nil, fmt.Errorf("%s: %w", pluginID, errInvalidPluginID)
		}
		pid = pluginID
	}
	rpath := "/run"
	if pid != "" {
		rpath += "/" + pid
	}

	data, err := c.get(ctx, rpath)
	if err != nil {
		return nil, err
	}

	var v Metrics
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("json parse - metrics: %w", err)
	}

	return &v, nil
}
