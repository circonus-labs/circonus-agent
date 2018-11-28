// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import "github.com/circonus-labs/go-apiclient"

//go:generate moq -out api_test.go . API

// API interface abstraction of circonus api (for mocking)
type API interface {
	Get(url string) ([]byte, error)
	FetchBroker(cid apiclient.CIDType) (*apiclient.Broker, error)
	FetchBrokers() (*[]apiclient.Broker, error)
	CreateCheckBundle(cfg *apiclient.CheckBundle) (*apiclient.CheckBundle, error)
	FetchCheckBundleMetrics(cid apiclient.CIDType) (*apiclient.CheckBundleMetrics, error)
	FetchCheckBundle(cid apiclient.CIDType) (*apiclient.CheckBundle, error)
	SearchCheckBundles(searchCriteria *apiclient.SearchQueryType, filterCriteria *apiclient.SearchFilterType) (*[]apiclient.CheckBundle, error)
	UpdateCheckBundle(cfg *apiclient.CheckBundle) (*apiclient.CheckBundle, error)
	UpdateCheckBundleMetrics(cfg *apiclient.CheckBundleMetrics) (*apiclient.CheckBundleMetrics, error)
}
