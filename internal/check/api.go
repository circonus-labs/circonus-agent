// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

//go:generate moq -out api_test.go . API

import "github.com/circonus-labs/circonus-gometrics/api"

// API interface abstraction of circonus api (for mocking)
type API interface {
	Get(url string) ([]byte, error)
	FetchBroker(cid api.CIDType) (*api.Broker, error)
	FetchBrokers() (*[]api.Broker, error)
	CreateCheckBundle(cfg *api.CheckBundle) (*api.CheckBundle, error)
	FetchCheckBundleMetrics(cid api.CIDType) (*api.CheckBundleMetrics, error)
	FetchCheckBundle(cid api.CIDType) (*api.CheckBundle, error)
	SearchCheckBundles(searchCriteria *api.SearchQueryType, filterCriteria *map[string][]string) (*[]api.CheckBundle, error)
	UpdateCheckBundle(cfg *api.CheckBundle) (*api.CheckBundle, error)
	UpdateCheckBundleMetrics(cfg *api.CheckBundleMetrics) (*api.CheckBundleMetrics, error)
}
