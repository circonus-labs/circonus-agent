// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
)

func (c *Check) loadState() (*metricStates, error) {
	if c.stateFile == "" {
		return nil, errors.New("invalid state file (empty)")
	}

	var ms metricStates

	sf, err := os.Open(c.stateFile)
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

func (c *Check) saveState(ms *metricStates) error {
	if c.stateFile == "" {
		return errors.New("invalid state file (empty)")
	}

	sf, err := ioutil.TempFile(c.statePath, "state")
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
	if err := os.Rename(sf.Name(), c.stateFile); err != nil {
		os.Remove(sf.Name())
		return errors.Wrap(err, "updating state file (removing temp file)")
	}

	return nil
}

func (c *Check) verifyStatePath() (bool, error) {
	if c.statePath == "" {
		return false, errors.New("invalid state path (empty)")
	}

	fs, err := os.Stat(c.statePath)
	if err != nil {
		return false, errors.Wrap(err, "stat state path")
	}

	if !fs.IsDir() {
		return false, errors.Errorf("state path is not a directory (%s)", c.statePath)
	}

	tf, err := ioutil.TempFile(c.statePath, "verify")
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
