// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	"fmt"
	stdlog "log"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-gometrics/api"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func getCheckConfig() (string, *url.URL, error) {
	cfg := &api.Config{
		TokenKey: viper.GetString(config.KeyAPITokenKey),
		TokenApp: viper.GetString(config.KeyAPITokenApp),
		URL:      viper.GetString(config.KeyAPIURL),
		Log:      stdlog.New(logger.With().Str("pkg", "circonus-gometrics.api").Logger(), "", 0),
		Debug:    viper.GetBool(config.KeyDebugCGM),
	}

	client, err := api.New(cfg)
	if err != nil {
		return "", nil, errors.Wrap(err, "Initializing cgm API")
	}

	bundle, err := getCheckBundle(client)
	if err != nil {
		return "", nil, err
	}

	if len(bundle.ReverseConnectURLs) == 0 {
		return "", nil, errors.New("No reverse URLs found in check")
	}
	rURL := bundle.ReverseConnectURLs[0]
	rSecret := bundle.Config["reverse:secret_key"]

	if rSecret != "" {
		rURL += "#" + rSecret
	}

	// Replace protocol, url.Parse does not understand 'mtev_reverse'.
	// Important part is validating what's after 'proto://'.
	// Using raw tls connections, the url protocol is not germane.
	reverseURL, err := url.Parse(strings.Replace(rURL, "mtev_reverse", "http", -1))
	if err != nil {
		return "", nil, errors.Wrapf(err, "Unable to parse reverse URL (%s)", rURL)
	}

	if len(bundle.Brokers) == 0 {
		return "", nil, errors.New("No brokers found in check")
	}
	brokerID := bundle.Brokers[0]

	return brokerID, reverseURL, nil
}

func getCheckBundle(client *api.API) (*api.CheckBundle, error) {
	var (
		bundle *api.CheckBundle
		err    error
	)

	cid := viper.GetString(config.KeyReverseCID)

	if cid != "" {
		checkBundleID := cid
		// Retrieve check bundle if we have a CID
		if ok, _ := regexp.MatchString("^[0-9]+$", checkBundleID); ok {
			checkBundleID = "/check_bundle/" + checkBundleID
		}
		bundle, err = client.FetchCheckBundle(api.CIDType(&checkBundleID))
		if err != nil {
			return nil, err
		}
	} else {
		// Otherwise, search for a check bundle
		bundle, err = searchForCheckBundle(client)
		if err != nil {
			return nil, err
		}
	}

	if bundle == nil {
		return nil, errors.New("No available check bundle to use for reverse")
	}

	if bundle.CID != cid {
		viper.Set(config.KeyReverseCID, bundle.CID)
	}

	return bundle, nil
}

func searchForCheckBundle(client *api.API) (*api.CheckBundle, error) {
	target := viper.GetString(config.KeyReverseTarget)
	if target == "" {
		host, err := os.Hostname()
		if err != nil {
			return nil, errors.Wrap(err, "Target not set, unable to derive valid hostname")
		}
		logger.Info().
			Str("hostname", host).
			Msg("Target not set, using hostname")
		target = host
	}

	criteria := api.SearchQueryType(fmt.Sprintf(`(active:1)(type:"json:nad")(target:"%s")`, target))

	bundles, err := client.SearchCheckBundles(&criteria, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Searching for check bundles")
	}

	if len(*bundles) == 0 {
		return nil, errors.Errorf("No check bundles matched criteria (%s)", string(criteria))
	}

	if len(*bundles) > 1 {
		return nil, errors.Errorf("More than one (%d) check bundle matched criteria (%s)", len(*bundles), string(criteria))
	}

	bundle := (*bundles)[0]

	return &bundle, nil
}

// func updateConfigFromCheckBundle(bundle *api.CheckBundle) error {
// 	if len(bundle.ReverseConnectURLs) == 0 {
// 		return errors.New("No reverse URLs found in check")
// 	}
// 	rURL := bundle.ReverseConnectURLs[0]
// 	rSecret := bundle.Config["reverse:secret_key"]
//
// 	if rSecret != "" {
// 		rURL += "#" + rSecret
// 	}
//
// 	// Replace protocol, url.Parse does not understand 'mtev_reverse'.
// 	// Important part is validating what's after 'proto://'. Using
// 	// a raw tls connection, the url protocol is not germane.
// 	r, err := url.Parse(strings.Replace(rURL, "mtev_reverse", "http", -1))
// 	if err != nil {
// 		return errors.Wrapf(err, "Unable to parse reverse URL (%s)", rURL)
// 	}
// 	reverseURL = r
//
// 	if len(bundle.Brokers) == 0 {
// 		return errors.New("No brokers found in check")
// 	}
// 	brokerID = bundle.Brokers[0]
//
// 	return nil
// }
