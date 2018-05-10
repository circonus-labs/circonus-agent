// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package api

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// Metrics retrieves metrics from one or all plugins
// NOTE: because the API is using the regular agent URL - the
//       agent will act as though any other client (e.g. a broker)
//       were requesting metrics - it will *run* the plugin(s).
func (c *Client) Metrics(pluginID string) (*Metrics, error) {
	pid := ""
	if pluginID != "" {
		if !c.pidVal.MatchString(pluginID) {
			return nil, errors.Errorf("invalid plugin id (%s)", pluginID)
		}
		pid = pluginID
	}
	rpath := "/run"
	if pid != "" {
		rpath += "/" + pid
	}

	data, err := c.get(rpath)
	if err != nil {
		return nil, err
	}

	var v Metrics
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, errors.Wrap(err, "parsing metrics")
	}

	return &v, nil
}
