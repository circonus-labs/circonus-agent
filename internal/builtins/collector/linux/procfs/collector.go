// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package procfs

import (
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	cgm "github.com/circonus-labs/circonus-gometrics"
)

// Define stubs to satisfy the collector.Collector interface.
//
// The individual wmi collector implementations must override Collect and Flush.
//
// ID and Inventory are generic and do not need to be overriden unless the
// collector implementation requires it.

// Collect returns collector metrics
func (c *pfscommon) Collect() error {
	c.Lock()
	defer c.Unlock()
	return collector.ErrNotImplemented
}

// Flush returns last metrics collected
func (c *pfscommon) Flush() cgm.Metrics {
	c.Lock()
	defer c.Unlock()
	if c.lastMetrics == nil {
		c.lastMetrics = cgm.Metrics{}
	}
	return c.lastMetrics
}

// ID returns the id of the instance
func (c *pfscommon) ID() string {
	c.Lock()
	defer c.Unlock()
	return c.id
}

// Inventory returns collector stats for /inventory endpoint
func (c *pfscommon) Inventory() collector.InventoryStats {
	c.Lock()
	defer c.Unlock()
	return collector.InventoryStats{
		ID:              c.id,
		LastRunStart:    c.lastStart.Format(time.RFC3339Nano),
		LastRunEnd:      c.lastEnd.Format(time.RFC3339Nano),
		LastRunDuration: c.lastRunDuration.String(),
		LastError:       c.lastError,
	}
}
