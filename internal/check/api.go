// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"github.com/circonus-labs/go-apiclient"
)

// API interface abstraction of circonus api (for mocking)
type API interface {
	CreateCheckBundle(cfg *apiclient.CheckBundle) (*apiclient.CheckBundle, error)
	FetchBroker(cid apiclient.CIDType) (*apiclient.Broker, error)
	FetchBrokers() (*[]apiclient.Broker, error)
	FetchCheck(cid apiclient.CIDType) (*apiclient.Check, error)
	FetchCheckBundle(cid apiclient.CIDType) (*apiclient.CheckBundle, error)
	FetchCheckBundleMetrics(cid apiclient.CIDType) (*apiclient.CheckBundleMetrics, error)
	Get(url string) ([]byte, error)
	SearchCheckBundles(searchCriteria *apiclient.SearchQueryType, filterCriteria *apiclient.SearchFilterType) (*[]apiclient.CheckBundle, error)
	UpdateCheckBundle(cfg *apiclient.CheckBundle) (*apiclient.CheckBundle, error)
	UpdateCheckBundleMetrics(cfg *apiclient.CheckBundleMetrics) (*apiclient.CheckBundleMetrics, error)
}
