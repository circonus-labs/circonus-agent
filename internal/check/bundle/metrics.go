// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package bundle

import (
	"encoding/json"
	"reflect"
	"strings"
	"time"

	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/go-apiclient"
	"github.com/pkg/errors"
)

// metricStates holds the status of known metrics persisted to metrics.json in defaults.StatePath
type metricStates map[string]string

//
// NOTE: manually managing metrics is deprecated, allow/deny filters should
//       be used going forward. Methods related to metric management will
//       be removed in the future.
//

// EnableNewMetrics updates the check bundle enabling any new metrics
func (cb *Bundle) EnableNewMetrics(m *cgm.Metrics) error {
	cb.Lock()
	defer cb.Unlock()

	if !cb.manage {
		return nil
	}

	if cb.bundle == nil {
		return ErrUninitialized
	}

	if !cb.metricStateUpdate {
		// let first submission of metrics go through if no state file
		// use case where agent is replacing an existing nad install (check already exists)
		if cb.metricStates == nil {
			cb.logger.Debug().Msg("no existing metric states, triggering load")
			cb.metricStateUpdate = true
			return nil
		}

		if time.Since(cb.lastRefresh) > cb.refreshTTL {
			cb.logger.Debug().
				Dur("since_last", time.Since(cb.lastRefresh)).
				Dur("ttl", cb.refreshTTL).
				Msg("TTL triggering metric state refresh")
			cb.metricStateUpdate = true
		}
	}

	if cb.metricStateUpdate {
		err := cb.setMetricStates(nil)
		if err != nil {
			return errors.Wrap(err, "updating metric states")
		}
	}

	cb.logger.Debug().Msg("scanning for new metrics")

	newMetrics := map[string]apiclient.CheckBundleMetric{}

	for mn, mv := range *m {
		if _, known := (*cb.metricStates)[mn]; !known {
			newMetrics[mn] = cb.configMetric(mn, mv)
			cb.logger.Debug().
				Interface("metric", newMetrics[mn]).
				Interface("mv", mv).
				Msg("found new metric")
		}
	}

	if len(newMetrics) > 0 {
		if err := cb.updateCheckBundleMetrics(&newMetrics); err != nil {
			cb.logger.Error().
				Err(err).
				Msg("adding new metrics to check bundle")
		}
	}

	return nil
}

func (cb *Bundle) getFullCheckMetrics() (*[]apiclient.CheckBundleMetric, error) {
	cbmPath := strings.Replace(cb.bundle.CID, "check_bundle", "check_bundle_metrics", -1)
	cbmPath += "?query_broker=1" // force for full set of metrics (active and available)

	data, err := cb.client.Get(cbmPath)
	if err != nil {
		return nil, errors.Wrap(err, "fetching check bundle metrics")
	}

	var metrics apiclient.CheckBundleMetrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return nil, errors.Wrap(err, "parsing check bundle metrics")
	}

	return &metrics.Metrics, nil
}

func (cb *Bundle) updateCheckBundleMetrics(m *map[string]apiclient.CheckBundleMetric) error {
	if m == nil {
		return errors.New("nil metrics passed to update")
	}

	// short circuit if no metrics to update
	if len(*m) == 0 {
		return nil
	}

	cid := cb.bundle.CID
	bundle, err := cb.client.FetchCheckBundle(apiclient.CIDType(&cid))
	if err != nil {
		return errors.Wrap(err, "unable to fetch up-to-date copy of check")
	}

	metrics := make([]apiclient.CheckBundleMetric, 0, len(*m))

	for mn, mv := range *m {
		cb.logger.Debug().
			Str("name", mn).
			Msg("configuring new check bundle metric")
		metrics = append(metrics, mv)
	}

	bundle.Metrics = append(bundle.Metrics, metrics...)

	cb.logger.Debug().Msg("updating check bundle with new metrics")
	newBundle, err := cb.client.UpdateCheckBundle(bundle)
	if err != nil {
		return errors.Wrap(err, "unable to update check bundle with new metrics")
	}

	if err := cb.setMetricStates(&newBundle.Metrics); err != nil {
		return errors.Wrap(err, "updating metrics states after enable new metrics")
	}

	cb.bundle = newBundle
	cb.bundle.Metrics = []apiclient.CheckBundleMetric{}

	return nil
}

func (cb *Bundle) configMetric(mn string, mv cgm.Metric) apiclient.CheckBundleMetric {
	cm := apiclient.CheckBundleMetric{
		Name:   mn,
		Status: cb.statusActiveMetric,
	}

	mtype := "numeric" // default
	switch mv.Type {
	case "n":
		vt := reflect.TypeOf(mv.Value).Kind().String()
		cb.logger.Debug().
			Str("mn", mn).
			Interface("mv", mv).
			Str("reflect_type", vt).
			Msg("circ type n")
		if vt == "slice" || vt == "array" {
			mtype = "histogram"
		}
	case "s":
		mtype = "text"
	}

	cm.Type = mtype

	return cm
}
