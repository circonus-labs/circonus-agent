package bundle

import (
	"encoding/json"
	"io/ioutil"
	"strings"

	"github.com/circonus-labs/go-apiclient"
	"github.com/gojuno/minimock/v3"
	"github.com/pkg/errors"
)

type pkicacert struct {
	Contents string `json:"contents"`
}

var (
	testCheckBundle apiclient.CheckBundle
	testBroker      apiclient.Broker
	cacert          pkicacert
)

func init() {
	if data, err := ioutil.ReadFile("testdata/checkbundle1234.json"); err != nil {
		panic(err)
	} else if err := json.Unmarshal(data, &testCheckBundle); err != nil {
		panic(err)

	}

	if data, err := ioutil.ReadFile("testdata/broker1234.json"); err != nil {
		panic(err)
	} else if err := json.Unmarshal(data, &testBroker); err != nil {
		panic(err)
	}

	if data, err := ioutil.ReadFile("testdata/ca.crt"); err != nil {
		panic(err)
	} else {
		cacert.Contents = string(data)
	}
}

func genMockClient(mc *minimock.Controller) *APIMock {

	m := NewAPIMock(mc)

	m.CreateCheckBundleMock.Set(func(cfg *apiclient.CheckBundle) (*apiclient.CheckBundle, error) {
		panic("TODO: mock CreateCheckBundle method")
	})

	m.FetchBrokerMock.Set(func(cid apiclient.CIDType) (*apiclient.Broker, error) {
		switch *cid {
		case "/broker/000":
			return nil, errors.New("forced mock api call error")
		case "/broker/123":
			return &apiclient.Broker{
				CID:  "/broker/123",
				Name: "foo",
				Type: "xxx",
				Details: []apiclient.BrokerDetail{
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
			return &apiclient.Broker{
				CID:  "/broker/456",
				Name: "bar",
				Type: "yyy",
				Details: []apiclient.BrokerDetail{
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
	})

	m.FetchBrokersMock.Return(&[]apiclient.Broker{
		{CID: "/broker/123", Name: "foo", Type: "circonus"},
		{CID: "/broker/456", Name: "bar", Type: "enterprise"},
		{CID: "/broker/789", Name: "baz", Type: "circonus"},
	}, nil)

	m.FetchCheckMock.Set(func(cid apiclient.CIDType) (*apiclient.Check, error) {
		panic("TODO: mock FetchCheck method")
	})

	m.FetchCheckBundleMock.Set(func(cid apiclient.CIDType) (*apiclient.CheckBundle, error) {
		switch *cid {
		case "/check_bundle/000":
			return nil, errors.New("forced mock api call error")
		case "/check_bundle/0002":
			x := testCheckBundle
			x.CID = *cid
			return &x, nil
		case "/check_bundle/1234":
			return &testCheckBundle, nil
		default:
			return nil, errors.Errorf("bad request cid (%s)", *cid)
		}
	})

	m.FetchCheckBundleMetricsMock.Set(func(cid apiclient.CIDType) (*apiclient.CheckBundleMetrics, error) {
		panic("TODO: mock FetchCheckBundleMetrics method")
	})

	m.GetMock.Set(func(url string) ([]byte, error) {
		switch url {
		case "/pki/ca.crt":
			ret, err := json.Marshal(cacert)
			if err != nil {
				panic(err)
			}
			return ret, nil
		case "/check_bundle_metrics/000?query_broker=1":
			return nil, errors.New("forced mock api call error")
		case "/check_bundle_metrics/0001?query_broker=1":
			return []byte("{"), nil
		case "/check_bundle_metrics/1234?query_broker=1":
			m := apiclient.CheckBundleMetrics{
				CID: "/check_bundle_metrics/1234",
				Metrics: []apiclient.CheckBundleMetric{
					{Name: "foo", Type: "n", Status: "active"},
				},
			}
			data, err := json.Marshal(m)
			if err != nil {
				panic(err)
			}
			return data, nil
		default:
			return nil, errors.Errorf("bad apiclient.Get(%s), no handler for url", url)
		}
	})

	m.SearchCheckBundlesMock.Set(func(searchCriteria *apiclient.SearchQueryType, filterCriteria *apiclient.SearchFilterType) (*[]apiclient.CheckBundle, error) {
		if strings.Contains(string(*searchCriteria), `target:"000"`) {
			return nil, errors.New("forced mock api call error")
		}
		if strings.Contains(string(*searchCriteria), `target:"not_found"`) {
			return &[]apiclient.CheckBundle{}, nil
		}
		if strings.Contains(string(*searchCriteria), `target:"multiple"`) {
			return &[]apiclient.CheckBundle{testCheckBundle, testCheckBundle}, nil
		}
		if strings.Contains(string(*searchCriteria), `target:"valid"`) {
			return &[]apiclient.CheckBundle{testCheckBundle}, nil
		}
		return nil, errors.Errorf("don't know what to do with search criteria (%s)", string(*searchCriteria))
	})

	m.UpdateCheckBundleMock.Set(func(cfg *apiclient.CheckBundle) (*apiclient.CheckBundle, error) {
		switch cfg.CID {
		case "/check_bundle/1234":
			return cfg, nil
		case "/check_bundle/0002":
			return nil, errors.New("api update check bundle error")
		default:
			return nil, errors.Errorf("add handler for %s", cfg.CID)
		}
	})

	m.UpdateCheckBundleMetricsMock.Set(func(cfg *apiclient.CheckBundleMetrics) (*apiclient.CheckBundleMetrics, error) {
		panic("TODO: mock UpdateCheckBundleMetrics method")
	})

	return m
}
