// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"encoding/json"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-gometrics/api"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

type pkicacert struct {
	Contents string `json:"contents"`
}

var (
	testCheckBundle api.CheckBundle
	testBroker      api.Broker
	cacert          pkicacert
)

func init() {
	if data, err := ioutil.ReadFile("testdata/check1234.json"); err != nil {
		panic(err)
	} else {
		if err := json.Unmarshal(data, &testCheckBundle); err != nil {
			panic(err)
		}
	}

	if data, err := ioutil.ReadFile("testdata/broker1234.json"); err != nil {
		panic(err)
	} else {
		if err := json.Unmarshal(data, &testBroker); err != nil {
			panic(err)
		}
	}

	if data, err := ioutil.ReadFile("testdata/ca.crt"); err != nil {
		panic(err)
	} else {
		cacert.Contents = string(data)
	}
}

func genMockClient() *APIMock {
	return &APIMock{
		CreateCheckBundleFunc: func(cfg *api.CheckBundle) (*api.CheckBundle, error) {
			panic("TODO: mock out the CreateCheckBundle method")
		},

		FetchBrokerFunc: func(cid api.CIDType) (*api.Broker, error) {
			switch *cid {
			case "/broker/000":
				return nil, errors.New("forced mock api call error")
			case "/broker/123":
				return &api.Broker{
					CID:  "/broker/123",
					Name: "foo",
					Type: "xxx",
					Details: []api.BrokerDetail{
						{
							Status:  "active",
							Modules: []string{"abc", "selfcheck", "hidden:abc123", "abcdef", "abcdefghi", "abcdefghijkl", "abcdefghijklmnopqrstu"},
						},
						{
							Status: "foobar",
						},
					},
				}, nil
			case "/broker/456":
				return &api.Broker{
					CID:  "/broker/456",
					Name: "bar",
					Type: "yyy",
					Details: []api.BrokerDetail{
						{
							Status: "foobar",
						},
					},
				}, nil
			case "/broker/1234":
				return &testBroker, nil
			default:
				return nil, errors.Errorf("bad broker request cid (%s)", *cid)
			}
		},

		FetchBrokersFunc: func() (*[]api.Broker, error) {
			return &[]api.Broker{
				{CID: "/broker/123", Name: "foo", Type: "circonus"},
				{CID: "/broker/456", Name: "bar", Type: "enterprise"},
				{CID: "/broker/789", Name: "baz", Type: "circonus"},
			}, nil
		},

		FetchCheckBundleFunc: func(cid api.CIDType) (*api.CheckBundle, error) {
			switch *cid {
			case "/check_bundle/000":
				return nil, errors.New("forced mock api call error")
			case "/check_bundle/1234":
				return &testCheckBundle, nil
			default:
				return nil, errors.Errorf("bad request cid (%s)", *cid)
			}
		},

		FetchCheckBundleMetricsFunc: func(cid api.CIDType) (*api.CheckBundleMetrics, error) {
			panic("TODO: mock out the FetchCheckBundleMetrics method")
		},

		GetFunc: func(url string) ([]byte, error) {
			switch url {
			case "/pki/ca.crt":
				ret, err := json.Marshal(cacert)
				if err != nil {
					panic(err)
				}
				return ret, nil
			}
			panic("TODO: mock out the Get method for " + url)
		},

		SearchCheckBundlesFunc: func(searchCriteria *api.SearchQueryType, filterCriteria *map[string][]string) (*[]api.CheckBundle, error) {
			if strings.Contains(string(*searchCriteria), `target:"000"`) {
				return nil, errors.New("forced mock api call error")
			}
			if strings.Contains(string(*searchCriteria), `target:"not_found"`) {
				return &[]api.CheckBundle{}, nil
			}
			if strings.Contains(string(*searchCriteria), `target:"multiple"`) {
				return &[]api.CheckBundle{testCheckBundle, testCheckBundle}, nil
			}
			if strings.Contains(string(*searchCriteria), `target:"valid"`) {
				return &[]api.CheckBundle{testCheckBundle}, nil
			}
			return nil, errors.Errorf("don't know what to do with search criteria (%s)", string(*searchCriteria))
		},

		UpdateCheckBundleFunc: func(cfg *api.CheckBundle) (*api.CheckBundle, error) {
			panic("TODO: mock out the UpdateCheckBundle method")
		},

		UpdateCheckBundleMetricsFunc: func(cfg *api.CheckBundleMetrics) (*api.CheckBundleMetrics, error) {
			panic("TODO: mock out the UpdateCheckBundleMetrics method")
		},
	}
}

//
// start actual tests for methods in main
//

func TestNew(t *testing.T) {
	t.Log("Testing New")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("check not needed")
	{
		viper.Reset()
		viper.Set(config.KeyCheckBundleID, "")
		viper.Set(config.KeyCheckCreate, false)
		viper.Set(config.KeyCheckEnableNewMetrics, false)
		viper.Set(config.KeyReverse, false)
		viper.Set(config.KeyAPITokenKey, "")
		viper.Set(config.KeyAPITokenApp, "")
		viper.Set(config.KeyAPIURL, "")

		_, err := New(nil)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}
}
