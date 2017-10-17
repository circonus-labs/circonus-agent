// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package cosi

// APIConfig defines the api configuraiton settings
type APIConfig struct {
	Key string
	App string
	URL string
}

// checkConfig defines the portion of check configuraiton to extract
type checkConfig struct {
	CID string `json:"_cid"`
}

// cosiConfig defines the api portion of the cosi configuration
type cosiConfig struct {
	APIKey string `json:"api_key"`
	APIApp string `json:"api_app"`
	APIURL string `json:"api_url"`
}

const (
	cosiName = "cosi"
)
