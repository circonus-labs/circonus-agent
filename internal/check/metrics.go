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
	"github.com/circonus-labs/go-apiclient"
	"github.com/pkg/errors"
)

func (c *Check) getFullCheckMetrics() (*[]apiclient.CheckBundleMetric, error) {
	cbmPath := strings.Replace(c.bundle.CID, "check_bundle", "check_bundle_metrics", -1)
	cbmPath += "?query_broker=1" // force for full set of metrics (active and available)

	data, err := c.client.Get(cbmPath)
	if err != nil {
		return nil, errors.Wrap(err, "fetching check bundle metrics")
	}

	var metrics apiclient.CheckBundleMetrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return nil, errors.Wrap(err, "parsing check bundle metrics")
	}

	return &metrics.Metrics, nil
}

func (c *Check) updateCheckBundleMetrics(m *map[string]apiclient.CheckBundleMetric) error {
	if m == nil {
		return errors.New("nil metrics passed to update")
	}

	// short circuit if no metrics to update
	if len(*m) == 0 {
		return nil
	}

	cid := c.bundle.CID
	bundle, err := c.client.FetchCheckBundle(apiclient.CIDType(&cid))
	if err != nil {
		return errors.Wrap(err, "unable to fetch up-to-date copy of check")
	}

	metrics := make([]apiclient.CheckBundleMetric, 0, len(*m))

	for mn, mv := range *m {
		c.logger.Debug().Str("name", mn).Msg("configuring new check bundle metric")
		metrics = append(metrics, mv)
	}

	bundle.Metrics = append(bundle.Metrics, metrics...)

	c.logger.Debug().Msg("updating check bundle with new metrics")
	newBundle, err := c.client.UpdateCheckBundle(bundle)
	if err != nil {
		return errors.Wrap(err, "unable to update check bundle with new metrics")
	}

	if err := c.setMetricStates(&newBundle.Metrics); err != nil {
		return errors.Wrap(err, "updating metrics states after enable new metrics")
	}

	c.bundle = newBundle
	c.bundle.Metrics = []apiclient.CheckBundleMetric{}

	return nil
}

func (c *Check) configMetric(mn string, mv cgm.Metric) apiclient.CheckBundleMetric {

	cm := apiclient.CheckBundleMetric{
		Name:   mn,
		Status: c.statusActiveMetric,
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
