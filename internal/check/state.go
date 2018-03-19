// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/pkg/errors"
)

func (c *Check) loadState() (*metricStates, error) {
	stateFile := filepath.Join(defaults.StatePath, "metrics.json")
	data, err := ioutil.ReadFile(stateFile)
	if err != nil {
		return nil, errors.Wrap(err, "loading metric state file")
	}

	var ms metricStates

	if err := json.Unmarshal(data, &ms); err != nil {
		return nil, errors.Wrap(err, "parsing metric state file")
	}

	return &ms, nil
}

func (c *Check) saveState(ms *metricStates) error {
	stateFile := filepath.Join(defaults.StatePath, "metrics.json")
	data, err := json.MarshalIndent(*ms, "", "  ")
	if err != nil {
		return errors.Wrap(err, "converting metric states to json")
	}

	if err := ioutil.WriteFile(stateFile, data, 0644); err != nil {
		return errors.Wrap(err, "saving metric state file")
	}

	return nil
}

func (c *Check) verifyStatePath() bool {
	fs, err := os.Stat(defaults.StatePath)
	if err != nil {
		c.logger.Error().Err(err).Str("state_path", defaults.StatePath).Msg("accessing state path")
		return false
	}
	if !fs.IsDir() {
		c.logger.Error().Str("state_path", defaults.StatePath).Msg("state path is not a directory")
		return false
	}
	testFile := filepath.Join(defaults.StatePath, "test.file")
	if err := ioutil.WriteFile(testFile, []byte("test file, ok to delete"), 0644); err != nil {
		c.logger.Error().Err(err).Msg("creating test state file")
		return false
	}
	if err := os.Remove(testFile); err != nil {
		c.logger.Error().Err(err).Msg("removing test state file")
		return false
	}
	return true
}
