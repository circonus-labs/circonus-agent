// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package bundle

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"time"

	"github.com/circonus-labs/go-apiclient"
	"github.com/pkg/errors"
)

func (cb *Bundle) setMetricStates(m *[]apiclient.CheckBundleMetric) error {
	if m == nil {
		metrics, err := cb.getFullCheckMetrics()
		if err != nil {
			return errors.Wrap(err, "updating metric states")
		}
		m = metrics
	}

	if cb.metricStates == nil {
		cb.metricStates = &metricStates{}
	}

	for _, metric := range *m {
		(*cb.metricStates)[metric.Name] = metric.Status
	}

	cb.lastRefresh = time.Now()
	cb.metricStateUpdate = false
	if err := cb.saveState(cb.metricStates); err != nil {
		cb.logger.Warn().Err(err).Msg("saving metric states")
	}

	cb.logger.Debug().Int("metrics", len(*cb.metricStates)).Msg("updating metric states done")
	return nil
}

func (cb *Bundle) loadState() (*metricStates, error) {
	if cb.stateFile == "" {
		return nil, errors.New("invalid state file (empty)")
	}

	var ms metricStates

	sf, err := os.Open(cb.stateFile)
	if err != nil {
		return nil, errors.Wrap(err, "opening state file")
	}
	defer sf.Close()

	dec := json.NewDecoder(sf)
	if err := dec.Decode(&ms); err != nil {
		return nil, errors.Wrap(err, "parsing state file")
	}

	return &ms, nil
}

func (cb *Bundle) saveState(ms *metricStates) error {
	if cb.stateFile == "" {
		return errors.New("invalid state file (empty)")
	}

	sf, err := ioutil.TempFile(cb.statePath, "state")
	if err != nil {
		return errors.Wrap(err, "creating temp state file")
	}

	enc := json.NewEncoder(sf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(ms); err != nil {
		sf.Close()
		os.Remove(sf.Name())
		return errors.Wrap(err, "error encoding state (removing temp file)")
	}

	sf.Close()
	if err := os.Rename(sf.Name(), cb.stateFile); err != nil {
		os.Remove(sf.Name())
		return errors.Wrap(err, "updating state file (removing temp file)")
	}

	return nil
}

func (cb *Bundle) verifyStatePath() (bool, error) {
	if cb.statePath == "" {
		return false, errors.New("invalid state path (empty)")
	}

	fs, err := os.Stat(cb.statePath)
	if err != nil {
		return false, errors.Wrap(err, "stat state path")
	}

	if !fs.IsDir() {
		return false, errors.Errorf("state path is not a directory (%s)", cb.statePath)
	}

	tf, err := ioutil.TempFile(cb.statePath, "verify")
	if err != nil {
		return false, errors.Wrap(err, "creating test state file")
	}

	if _, err := tf.Write([]byte("test file, ok to delete")); err != nil {
		return false, errors.Wrapf(err, "writing test state file (%s)", tf.Name())
	}

	if err := tf.Close(); err != nil {
		return false, errors.Wrapf(err, "closing test state file (%s)", tf.Name())
	}

	if err := os.Remove(tf.Name()); err != nil {
		return false, errors.Wrapf(err, "removing test state file (%s)", tf.Name())
	}

	return true, nil
}
