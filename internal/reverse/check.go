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
	apiconf "github.com/circonus-labs/circonus-gometrics/api/config"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func (c *Connection) setCheckConfig() error {
	bid, reverseURL, err := c.getCheckConfig()
	if err != nil {
		return errors.Wrap(err, "reverse configuration (check)")
	}

	tlsConfig, err := c.getTLSConfig(bid, reverseURL)
	if err != nil {
		return errors.Wrap(err, "reverse configuration (tls)")
	}

	c.reverseURL = reverseURL
	c.tlsConfig = tlsConfig

	return nil
}

func (c *Connection) getCheckConfig() (string, *url.URL, error) {
	cfg := &api.Config{
		TokenKey: viper.GetString(config.KeyAPITokenKey),
		TokenApp: viper.GetString(config.KeyAPITokenApp),
		URL:      viper.GetString(config.KeyAPIURL),
		Log:      stdlog.New(c.logger.With().Str("pkg", "circonus-gometrics.api").Logger(), "", 0),
		Debug:    viper.GetBool(config.KeyDebugCGM),
	}

	client, err := api.New(cfg)
	if err != nil {
		return "", nil, errors.Wrap(err, "Initializing cgm API")
	}

	bundle, err := c.getCheckBundle(client)
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

func (c *Connection) getCheckBundle(client *api.API) (*api.CheckBundle, error) {
	var (
		bundle *api.CheckBundle
		err    error
	)

	if c.checkCID != "" {
		// Retrieve check bundle if we have a CID
		if ok, _ := regexp.MatchString(`^[0-9]+$`, c.checkCID); ok {
			c.checkCID = "/check_bundle/" + c.checkCID
		}
		bundle, err = client.FetchCheckBundle(api.CIDType(&c.checkCID))
		if err != nil {
			return nil, err
		}
	} else {
		// Otherwise, search for a check bundle (optionally, create one if not found)
		bundle, err = c.searchForCheckBundle(client)
		if err != nil {
			return nil, err
		}
	}

	if bundle == nil {
		return nil, errors.New("No available check bundle to use for reverse")
	}

	if bundle.CID != c.checkCID {
		c.checkCID = bundle.CID
	}

	return bundle, nil
}

func (c *Connection) searchForCheckBundle(client *api.API) (*api.CheckBundle, error) {
	target := viper.GetString(config.KeyReverseTarget)
	if target == "" {
		host, err := os.Hostname()
		if err != nil {
			return nil, errors.Wrap(err, "Target not set, unable to derive valid hostname")
		}
		c.logger.Info().
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
		if !viper.GetBool(config.KeyReverseCreateCheck) {
			return nil, errors.Errorf("No check bundles matched criteria (%s)", string(criteria))
		}

		bundle, err := c.createCheckBundle(client, target)
		if err != nil {
			return nil, err
		}
		return bundle, nil
	}

	if len(*bundles) > 1 {
		return nil, errors.Errorf("More than one (%d) check bundle matched criteria (%s)", len(*bundles), string(criteria))
	}

	bundle := (*bundles)[0]

	return &bundle, nil
}

func (c *Connection) createCheckBundle(client *api.API, target string) (*api.CheckBundle, error) {

	addr := c.agentAddress
	if addr[0:1] == ":" {
		addr = "localhost" + addr
	}
	cfg := api.NewCheckBundle()
	cfg.DisplayName = viper.GetString(config.KeyReverseCreateCheckTitle)
	cfg.Target = target
	cfg.Type = "json:nad"
	cfg.Config = api.CheckBundleConfig{apiconf.URL: "http://" + addr + "/"}
	cfg.Metrics = []api.CheckBundleMetric{
		api.CheckBundleMetric{Name: "placeholder", Type: "text", Status: "active"}, // one metric is required again
	}

	tags := viper.GetString(config.KeyReverseCreateCheckTags)
	if tags != "" {
		cfg.Tags = strings.Split(tags, ",")
	}

	brokerCID := viper.GetString(config.KeyReverseCreateCheckBroker)
	if brokerCID == "" || strings.ToLower(brokerCID) == "select" {
		broker, err := c.selectBroker(client, "json:nad")
		if err != nil {
			return nil, err
		}

		brokerCID = broker.CID
	}

	if ok, _ := regexp.MatchString(`^[0-9]+$`, brokerCID); ok {
		brokerCID = "/broker/" + brokerCID
	}

	cfg.Brokers = []string{brokerCID}

	bundle, err := client.CreateCheckBundle(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "creating check bundle")
	}

	return bundle, nil
}
