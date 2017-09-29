// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"encoding/json"
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// apiRequired checks to see if any options are set which would *require* accessing the API
func apiRequired() bool {
	// reverse connections require API access
	if viper.GetBool(KeyReverse) {
		return true
	}

	// statsd w/group check enabled require API access
	if !viper.GetBool(KeyStatsdDisabled) && viper.GetString(KeyStatsdGroupCID) != "" {
		return true
	}

	return false
}

func validateAPIOptions() error {
	if apiOK {
		return nil
	}

	apiKey := viper.GetString(KeyAPITokenKey)
	apiApp := viper.GetString(KeyAPITokenApp)
	apiURL := viper.GetString(KeyAPIURL)
	apiCAFile := viper.GetString(KeyAPICAFile)

	// if key is 'cosi' - load the cosi api config
	if strings.ToLower(apiKey) == cosiName {
		cKey, cApp, cURL, err := loadCOSIConfig()
		if err != nil {
			return err
		}

		apiKey = cKey
		apiApp = cApp
		apiURL = cURL
	}

	// API is required for reverse and/or statsd

	if apiKey == "" {
		return errors.New("API key is required")
	}

	if apiApp == "" {
		return errors.New("API app is required")
	}

	if apiURL == "" {
		return errors.New("API URL is required")
	}

	if apiURL != defaults.APIURL {
		parsedURL, err := url.Parse(apiURL)
		if err != nil {
			return errors.Wrap(err, "Invalid API URL")
		}
		if parsedURL.Scheme == "" || parsedURL.Host == "" || parsedURL.Path == "" {
			return errors.Errorf("Invalid API URL (%s)", apiURL)
		}
	}

	// NOTE the api ca file doesn't come from the cosi config
	if apiCAFile != "" {
		f, err := verifyFile(apiCAFile)
		if err != nil {
			return err
		}
		viper.Set(KeyAPICAFile, f)
	}

	viper.Set(KeyAPITokenKey, apiKey)
	viper.Set(KeyAPITokenApp, apiApp)
	viper.Set(KeyAPIURL, apiURL)
	apiOK = true

	return nil
}

type cosiConfig struct {
	APIKey string `json:"api_key"`
	APIApp string `json:"api_app"`
	APIURL string `json:"api_url"`
}

func loadCOSIConfig() (string, string, string, error) {
	data, err := ioutil.ReadFile(cosiCfgFile)
	if err != nil {
		return "", "", "", errors.Wrap(err, "Unable to access cosi config")
	}

	var cfg cosiConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", "", "", errors.Wrapf(err, "Unable to parse cosi config (%s)", cosiCfgFile)
	}

	if cfg.APIKey == "" {
		return "", "", "", errors.Errorf("Missing API key, invalid cosi config (%s)", cosiCfgFile)
	}
	if cfg.APIApp == "" {
		return "", "", "", errors.Errorf("Missing API app, invalid cosi config (%s)", cosiCfgFile)
	}
	if cfg.APIURL == "" {
		return "", "", "", errors.Errorf("Missing API URL, invalid cosi config (%s)", cosiCfgFile)
	}

	return cfg.APIKey, cfg.APIApp, cfg.APIURL, nil

}
