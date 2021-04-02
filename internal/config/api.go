// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/spf13/viper"
)

var (
	errAPIKeyRequired = fmt.Errorf("API key is required")
	errAPIAppRequired = fmt.Errorf("API app is required")
	errAPIURLRequired = fmt.Errorf("API URL is required")
)

// apiRequired checks to see if any options are set which would *require* accessing the API.
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
	apiKey := viper.GetString(KeyAPITokenKey)
	apiApp := viper.GetString(KeyAPITokenApp)
	apiURL := viper.GetString(KeyAPIURL)
	apiCAFile := viper.GetString(KeyAPICAFile)

	// if key is 'cosi' - load the cosi api config
	if strings.ToLower(apiKey) == cosiName {
		cfg, err := loadCosiAPIConfig()
		if err != nil {
			return err
		}

		apiKey = cfg.Key
		apiApp = cfg.App
		apiURL = cfg.URL
	}

	// API is required for reverse and/or statsd

	if apiKey == "" {
		return errAPIKeyRequired
	}

	if apiApp == "" {
		return errAPIAppRequired
	}

	if apiURL == "" {
		return errAPIURLRequired
	}

	if apiURL != defaults.APIURL {
		parsedURL, err := url.Parse(apiURL)
		if err != nil {
			return fmt.Errorf("invalid API URL: %w", err)
		}
		if parsedURL.Scheme == "" || parsedURL.Host == "" || parsedURL.Path == "" {
			return fmt.Errorf("invalid API URL (%s)", apiURL) //nolint:goerr113
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

	return nil
}
