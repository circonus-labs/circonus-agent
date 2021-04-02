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

// Inventory retrieves the active plugin inventory from the agent.
func (c *Client) Inventory() (*Inventory, error) {
	return c.InventoryWithContext(context.Background())
}

// InventoryWithContext retrieves the active plugin inventory from the agent.
func (c *Client) InventoryWithContext(ctx context.Context) (*Inventory, error) {
	data, err := c.get(ctx, "/inventory/")
	if err != nil {
		return nil, err
	}

	var v Inventory
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("json parse - inventory: %w", err)
	}

	return &v, nil
}
