// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package api

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// Inventory retrieves the active plugin inventory from the agent
func (c *Client) Inventory() (*Inventory, error) {
	data, err := c.get("/inventory/")
	if err != nil {
		return nil, err
	}

	var v Inventory
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, errors.Wrap(err, "parsing inventory")
	}

	return &v, nil
}
