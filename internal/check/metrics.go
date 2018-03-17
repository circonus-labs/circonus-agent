// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"encoding/json"
	"strings"

	"github.com/circonus-labs/circonus-gometrics/api"
	"github.com/pkg/errors"
)

func (c *Check) getFullCheckMetrics() ([]api.CheckBundleMetric, error) {
	cbmPath := strings.Replace(c.bundle.CID, "check_bundle", "check_bundle_metrics", -1)
	cbmPath += "?query_broker=1" // force for full set of metrics (active and available)

	data, err := c.client.Get(cbmPath)
	if err != nil {
		return nil, errors.Wrap(err, "fetching check bundle metrics")
	}

	var metrics api.CheckBundleMetrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return nil, errors.Wrap(err, "parsing check bundle metrics")
	}

	return metrics.Metrics, nil
}
