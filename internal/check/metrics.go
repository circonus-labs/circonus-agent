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

func (c *Check) updateCheckBundleMetrics(m *map[string]api.CheckBundleMetric) error {
	metrics := make([]api.CheckBundleMetric, 0, len(*m))

	for mn, mv := range *m {
		c.logger.Debug().Str("name", mn).Msg("configuring new check bundle metric")
		metrics = append(metrics, mv)
	}

	cfg := &api.CheckBundleMetrics{
		CID:     strings.Replace(c.bundle.CID, "check_bundle", "check_bundle_metrics", 1),
		Metrics: metrics,
	}

	c.logger.Debug().Interface("payload", cfg).Msg("sending new metrics to API")

	results, err := c.client.UpdateCheckBundleMetrics(cfg)
	if err != nil {
		return errors.Wrap(err, "enabling new metrics")
	}

	for _, ms := range results.Metrics {
		switch ms.Status {
		case "active":
			c.logger.Info().Str("metric", ms.Name).Msg("enabled")
		case "noop":
			c.logger.Info().Str("metric", ms.Name).Msg("already enabled")
		case "fail":
			c.logger.Info().Str("metric", ms.Name).Msg("could not enable")
		default:
			c.logger.Info().Str("metric", ms.Name).Str("status", ms.Status).Msg("unknown status")
		}
	}

	return nil
}
