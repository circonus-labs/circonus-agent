// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import "time"

func (c *Check) refreshMetrics() error {
	if !c.manage { // not managing metrics
		return nil
	}
	if c.refreshTTL == time.Duration(0) { // never refresh
		return nil
	}
	if c.metrics != nil && c.refreshTTL > time.Since(c.lastRefresh) {
		return nil
	}

	c.Lock()
	defer c.Unlock()

	// 1. fetch a new copy of the check bundle metrics using the API
	// 2. rebuild metric state list
	// 3. update last refresh timestamp
	c.lastRefresh = time.Now()

	return nil
}
