// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-gometrics/api"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func (c *Check) fetchCheck(cid string) error {
	if cid == "" {
		return errors.New("invalid cid (empty)")
	}

	if ok, _ := regexp.MatchString(`^[0-9]+$`, cid); ok {
		cid = "/check_bundle/" + cid
	}

	if ok, _ := regexp.MatchString(`^/check_bundle/[0-9]+$`, cid); !ok {
		return errors.Errorf("invalid cid (%s)", cid)
	}

	bundle, err := c.client.FetchCheckBundle(api.CIDType(&cid))
	if err != nil {
		return errors.Wrapf(err, "unable to retrieve check bundle (%s)", cid)
	}

	c.bundle = bundle

	return nil
}

func (c *Check) findCheck() (int, error) {
	target := viper.GetString(config.KeyCheckTarget)
	if target == "" {
		return -1, errors.New("invalid check target (empty)")
	}

	criteria := api.SearchQueryType(fmt.Sprintf(`(active:1)(type:"json:nad")(target:"%s")`, target))
	bundles, err := c.client.SearchCheckBundles(&criteria, nil)
	if err != nil {
		return -1, errors.Wrap(err, "searching for check bundle")
	}

	found := len(*bundles)

	if found == 0 {
		return found, errors.Errorf("no check bundles matched criteria (%s)", string(criteria))
	}

	if found > 1 {
		return found, errors.Errorf("more than one (%d) check bundle matched criteria (%s)", len(*bundles), string(criteria))
	}

	c.bundle = &(*bundles)[0]

	return found, nil
}

func (c *Check) createCheck() error {
	// addr := c.agentAddress
	// if addr[0:1] == ":" {
	// 	addr = "localhost" + addr
	// }
	target := viper.GetString(config.KeyCheckTarget)
	if target == "" {
		return errors.New("invalid check target (empty)")
	}

	cfg := api.NewCheckBundle()
	cfg.Target = target
	cfg.DisplayName = viper.GetString(config.KeyCheckTitle)
	if cfg.DisplayName == "" {
		cfg.DisplayName = cfg.Target + " /agent"
	}
	cfg.Type = "json:nad"
	// cfg.Config = api.CheckBundleConfig{apiconf.URL: "http://" + addr + "/"}
	// cfg.Config = api.CheckBundleConfig{apiconf.URL: "http://" + cfg.Target + "/"}
	cfg.Config = api.CheckBundleConfig{}
	cfg.Metrics = []api.CheckBundleMetric{
		{Name: "placeholder", Type: "text", Status: "active"}, // one metric is required again
	}

	tags := viper.GetString(config.KeyCheckTags)
	if tags != "" {
		cfg.Tags = strings.Split(tags, ",")
	}

	brokerCID := viper.GetString(config.KeyCheckBroker)
	if brokerCID == "" || strings.ToLower(brokerCID) == "select" {
		broker, err := c.selectBroker("json:nad")
		if err != nil {
			return errors.Wrap(err, "selecting broker to create check")
		}

		brokerCID = broker.CID
	}

	if ok, _ := regexp.MatchString(`^[0-9]+$`, brokerCID); ok {
		brokerCID = "/broker/" + brokerCID
	}

	cfg.Brokers = []string{brokerCID}

	bundle, err := c.client.CreateCheckBundle(cfg)
	if err != nil {
		return errors.Wrap(err, "creating check bundle")
	}

	c.bundle = bundle

	return nil
}
