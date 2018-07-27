// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"

	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/pkg/errors"
)

// APIConfig defines the api configuration settings
type APIConfig struct {
	Key string
	App string
	URL string
}

// checkConfig defines the portion of check configuration to extract
type checkConfig struct {
	CID string `json:"_cid"`
}

// cosiV1Config defines the api portion of the cosi configuration
type cosiV1Config struct {
	APIKey string `json:"api_key"`
	APIApp string `json:"api_app"`
	APIURL string `json:"api_url"`
}

// cosiV2API defines the api portion of the cosi v2 configuration
type cosiV2API struct {
	Key string `json:"key"`
	App string `json:"app"`
	URL string `json:"url"`
}

// cosiV2Config defines the cosi v2 configuration
type cosiV2Config struct {
	API cosiV2API `json:"api"`
}

// LoadCosiCheckID reads a cosi configuration to obtain the _cid
func LoadCosiCheckID(checkType string) (string, error) {
	if checkType != "system" && checkType != "group" {
		return "", errors.Errorf("unknown cosi check type (%s)", checkType)
	}
	return loadCheckConfig(filepath.Join(defaults.BasePath, "..", cosiName, "registration", fmt.Sprintf("registration-check-%s.json", checkType)))
}

// IsValidCheckID validates a check bundle id
func IsValidCheckID(cid string) (bool, error) {
	ok, err := regexp.MatchString("^(/check_bundle/)?[0-9]+$", cid)
	if err != nil {
		return false, errors.Wrapf(err, "regex issue validating Check ID (%s)", cid)
	}

	return ok, nil
}

// loadCOSIAPIConfig loads the Circonus API configuration used by cosi
func loadCosiAPIConfig() (*APIConfig, error) {
	var cfg *APIConfig
	var err error

	cfgFile := filepath.Join(defaults.BasePath, "..", cosiName, "etc", "cosi")
	cfg, err = loadCosiV1Config(cfgFile + ".json")
	if err != nil {
		cfg, err = loadCosiV2Config(cfgFile)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}
	return cfg, nil
}

// loadCosiV1Config loads (currently, only api) portion of cosi configuration
func loadCosiV1Config(cfgFile string) (*APIConfig, error) {
	data, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		return nil, errors.Wrap(err, "unable to access cosi config")
	}

	var cfg cosiV1Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, errors.Wrapf(err, "unable to parse cosi config (%s)", cfgFile)
	}

	if cfg.APIKey == "" {
		return nil, errors.Errorf("missing API key, invalid cosi config (%s)", cfgFile)
	}
	if cfg.APIApp == "" {
		return nil, errors.Errorf("missing API app, invalid cosi config (%s)", cfgFile)
	}
	if cfg.APIURL == "" {
		return nil, errors.Errorf("missing API URL, invalid cosi config (%s)", cfgFile)
	}

	return &APIConfig{
		Key: cfg.APIKey,
		App: cfg.APIApp,
		URL: cfg.APIURL,
	}, nil
}

// loadCosiV2Config loads (currently, only api) portion of cosi v2 configuration
func loadCosiV2Config(cfgFile string) (*APIConfig, error) {
	var cfg cosiV2Config
	err := LoadConfigFile(cfgFile, &cfg)
	if err != nil {
		return nil, errors.Wrap(err, "unable to load cosi config")
	}

	if cfg.API.Key == "" {
		return nil, errors.Errorf("missing API key, invalid cosi config (%s)", cfgFile)
	}
	if cfg.API.App == "" {
		return nil, errors.Errorf("missing API app, invalid cosi config (%s)", cfgFile)
	}
	if cfg.API.URL == "" {
		return nil, errors.Errorf("missing API URL, invalid cosi config (%s)", cfgFile)
	}

	return &APIConfig{
		Key: cfg.API.Key,
		App: cfg.API.App,
		URL: cfg.API.URL,
	}, nil
}

// loadChecKConfig loads (currently, only cid) portion of a cosi check config
func loadCheckConfig(cfgFile string) (string, error) {
	data, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		return "", errors.Wrap(err, "unable to access cosi check config")
	}

	var cfg checkConfig
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return "", errors.Wrapf(err, "unable to parse cosi check cosi config (%s)", cfgFile)
	}
	if cfg.CID == "" {
		return "", errors.Errorf("missing CID key, invalid cosi check config (%s)", cfgFile)
	}

	ok, err := IsValidCheckID(cfg.CID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", errors.Errorf("invalid Check ID (%s)", cfg.CID)
	}

	return cfg.CID, nil
}
