// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"encoding/json"
	"reflect"
	"strings"

	cgm "github.com/circonus-labs/circonus-gometrics"
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

// func (c *Check) updateCheckBundleMetrics(m *map[string]api.CheckBundleMetric) error {
// 	metrics := make([]api.CheckBundleMetric, 0, len(*m))
//
// 	for mn, mv := range *m {
// 		c.logger.Debug().Str("name", mn).Msg("configuring new check bundle metric")
// 		metrics = append(metrics, mv)
// 	}
//
// 	cfg := &api.CheckBundleMetrics{
// 		CID:     strings.Replace(c.bundle.CID, "check_bundle", "check_bundle_metrics", 1),
// 		Metrics: metrics,
// 	}
//
// 	c.logger.Debug().Interface("payload", cfg).Msg("sending new metrics to API")
//
// 	results, err := c.client.UpdateCheckBundleMetrics(cfg)
// 	if err != nil {
// 		return errors.Wrap(err, "enabling new metrics")
// 	}
//
// 	for _, ms := range results.Metrics {
// 		if ms.Result == nil {
// 			c.logger.Info().Interface("metric", ms).Msg("nil 'Result' field, unknown operation status")
// 			continue
// 		}
// 		switch *ms.Result {
// 		case "success":
// 			c.logger.Info().Str("metric", ms.Name).Msg("enabled")
// 		case "noop":
// 			c.logger.Info().Str("metric", ms.Name).Msg("already enabled")
// 		case "failure":
// 			c.logger.Info().Str("metric", ms.Name).Msg("could not enable")
// 		default:
// 			c.logger.Info().Str("metric", ms.Name).Str("result", *ms.Result).Msg("unknown result")
// 		}
// 	}
//
// 	return nil
// }

func (c *Check) updateCheckBundleMetrics(m *map[string]api.CheckBundleMetric) error {
	if m == nil {
		return errors.New("nil metrics passed to update")
	}

	// short circuit if no metrics to update
	if len(*m) == 0 {
		return nil
	}

	cid := c.bundle.CID
	bundle, err := c.client.FetchCheckBundle(api.CIDType(&cid))
	if err != nil {
		return errors.Wrap(err, "unable to fetch up-to-date copy of check")
	}

	metrics := make([]api.CheckBundleMetric, 0, len(*m))

	for mn, mv := range *m {
		c.logger.Debug().Str("name", mn).Msg("configuring new check bundle metric")
		metrics = append(metrics, mv)
	}

	bundle.Metrics = append(bundle.Metrics, metrics...)

	newBundle, err := c.client.UpdateCheckBundle(bundle)
	if err != nil {
		return errors.Wrap(err, "unable to update check bundle with new metrics")
	}

	c.bundle = newBundle

	return nil
}

func (c *Check) configMetric(mn string, mv cgm.Metric) api.CheckBundleMetric {

	cm := api.CheckBundleMetric{
		Name:   mn,
		Status: activeMetricStatus,
	}

	mtype := "numeric" // default
	switch mv.Type {
	case "n":
		vt := reflect.TypeOf(mv.Value).Kind().String()
		c.logger.Debug().Str("mn", mn).Interface("mv", mv).Str("reflect_type", vt).Msg("circ type n")
		if vt == "slice" || vt == "array" {
			mtype = "histogram"
		}
	case "s":
		mtype = "text"
	}

	cm.Type = mtype

	return cm
}
