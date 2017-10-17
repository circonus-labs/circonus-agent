// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package cosi

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"

	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/pkg/errors"
)

// LoadAPIConfig loads the Circonus API configuration used by cosi
func LoadAPIConfig() (*APIConfig, error) {
	return loadCosiConfig(filepath.Join(defaults.BasePath, "..", cosiName, "etc", "cosi.json"))
}

// LoadCheckID reads a cosi configuration to obtain the _cid
func LoadCheckID(checkType string) (string, error) {
	if checkType != "system" && checkType != "group" {
		return "", errors.Errorf("unknown cosi check type (%s)", checkType)
	}
	return loadCheckConfig(filepath.Join(defaults.BasePath, "..", cosiName, "registration", fmt.Sprintf("registration-check-%s.json", checkType)))
}

// ValidCheckID validates a check bundle id
func ValidCheckID(cid string) (bool, error) {
	ok, err := regexp.MatchString("^(/check_bundle/)?[0-9]+$", cid)
	if err != nil {
		return false, errors.Wrapf(err, "regex issue validating Check ID (%s)", cid)
	}

	return ok, nil
}

// loadCosiConfig loads (currently, only api) portion of cosi configuration
func loadCosiConfig(cfgFile string) (*APIConfig, error) {
	data, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to access cosi config")
	}

	var cfg cosiConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, errors.Wrapf(err, "Unable to parse cosi config (%s)", cfgFile)
	}

	if cfg.APIKey == "" {
		return nil, errors.Errorf("Missing API key, invalid cosi config (%s)", cfgFile)
	}
	if cfg.APIApp == "" {
		return nil, errors.Errorf("Missing API app, invalid cosi config (%s)", cfgFile)
	}
	if cfg.APIURL == "" {
		return nil, errors.Errorf("Missing API URL, invalid cosi config (%s)", cfgFile)
	}

	return &APIConfig{
		Key: cfg.APIKey,
		App: cfg.APIApp,
		URL: cfg.APIURL,
	}, nil
}

// loadChecKConfig loads (currently, only cid) portion of a cosi check config
func loadCheckConfig(cfgFile string) (string, error) {
	data, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		return "", errors.Wrap(err, "Unable to access cosi check config")
	}

	var cfg checkConfig
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return "", errors.Wrapf(err, "Unable to parse cosi check cosi config (%s)", cfgFile)
	}
	if cfg.CID == "" {
		return "", errors.Errorf("Missing CID key, invalid cosi check config (%s)", cfgFile)
	}

	ok, err := ValidCheckID(cfg.CID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", errors.New("Invalid Check ID")
	}

	return cfg.CID, nil
}
