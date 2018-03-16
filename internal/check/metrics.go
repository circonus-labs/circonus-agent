// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"strings"
	"time"

	"github.com/circonus-labs/circonus-gometrics/api"
	"github.com/pkg/errors"
)

func (c *Check) refreshMetrics() error {
	if !c.manage { // not managing metrics
		return nil
	}
	if c.bundle == nil {
		return errors.New("invalid state (bundle is nil)")
	}
	if c.refreshTTL == time.Duration(0) { // never refresh
		return nil
	}
	if c.metrics != nil && c.refreshTTL > time.Since(c.lastRefresh) {
		return nil
	}

	c.Lock()
	defer c.Unlock()

	cid := strings.Replace(c.bundle.CID, "check_bundle", "check_bundle_metrics", -1)

	metrics, err := c.client.FetchCheckBundleMetrics(api.CIDType(&cid))
	if err != nil {
		return errors.Wrap(err, "refresh check bundle metrics")
	}

	newMetrics := make(map[string]api.CheckBundleMetric)

	for _, m := range (*metrics).Metrics {
		newMetrics[m.Name] = m
	}

	c.metrics = &newMetrics
	c.lastRefresh = time.Now()

	return nil
}
