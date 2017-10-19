// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"net/url"
	"strings"

	"github.com/circonus-labs/circonus-agent/internal/config/cosi"
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
	apiKey := viper.GetString(KeyAPITokenKey)
	apiApp := viper.GetString(KeyAPITokenApp)
	apiURL := viper.GetString(KeyAPIURL)
	apiCAFile := viper.GetString(KeyAPICAFile)

	// if key is 'cosi' - load the cosi api config
	if strings.ToLower(apiKey) == cosiName {
		cfg, err := cosi.LoadAPIConfig()
		if err != nil {
			return err
		}

		apiKey = cfg.Key
		apiApp = cfg.App
		apiURL = cfg.URL
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

	return nil
}
