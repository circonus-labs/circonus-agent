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

func (c *Check) setCheck() error {
	// retrieve the check via the Circonus API or create a new check (if configured to do so)
	isCreate := viper.GetBool(config.KeyCheckCreate)
	isReverse := viper.GetBool(config.KeyReverse)
	cid := viper.GetString(config.KeyCheckBundleID)

	var bundle *api.CheckBundle

	// if explicit cid configured, attempt to fetch check bundle using cid
	if cid != "" {
		b, err := c.fetchCheck(cid)
		if err != nil {
			return errors.Wrapf(err, "fetching check for cid %s", cid)
		}
		bundle = b
	} else {
		// if no cid configured, attempt to find check bundle matching this system
		b, found, err := c.findCheck()
		if err != nil {
			if !isCreate || found != 0 {
				return errors.Wrap(err, "unable to find a check for this system")
			}
			c.logger.Info().Msg("no existing check found, creating")
			// attempt to create if not found and create flag ON
			b, err = c.createCheck()
			if err != nil {
				return errors.Wrap(err, "creating new check for this system")
			}
		}
		bundle = b
	}

	if bundle == nil {
		return errors.New("invalid Check object state, bundle is nil")
	}

	c.bundle = bundle
	c.bundle.Metrics = []api.CheckBundleMetric{}

	if isReverse {
		// populate reverse configuration
		err := c.setReverseConfig()
		if err != nil {
			return errors.Wrap(err, "setting up reverse configuration")
		}
	}

	return nil
}

func (c *Check) fetchCheck(cid string) (*api.CheckBundle, error) {
	if cid == "" {
		return nil, errors.New("invalid cid (empty)")
	}

	if ok, _ := regexp.MatchString(`^[0-9]+$`, cid); ok {
		cid = "/check_bundle/" + cid
	}

	if ok, _ := regexp.MatchString(`^/check_bundle/[0-9]+$`, cid); !ok {
		return nil, errors.Errorf("invalid cid (%s)", cid)
	}

	bundle, err := c.client.FetchCheckBundle(api.CIDType(&cid))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to retrieve check bundle (%s)", cid)
	}

	return bundle, nil
}

func (c *Check) findCheck() (*api.CheckBundle, int, error) {
	target := viper.GetString(config.KeyCheckTarget)
	if target == "" {
		return nil, -1, errors.New("invalid check target (empty)")
	}

	criteria := api.SearchQueryType(fmt.Sprintf(`(active:1)(type:"json:nad")(target:"%s")`, target))
	bundles, err := c.client.SearchCheckBundles(&criteria, nil)
	if err != nil {
		return nil, -1, errors.Wrap(err, "searching for check bundle")
	}

	found := len(*bundles)

	if found == 0 {
		return nil, found, errors.Errorf("no check bundles matched criteria (%s)", string(criteria))
	}

	if found > 1 {
		return nil, found, errors.Errorf("more than one (%d) check bundle matched criteria (%s)", len(*bundles), string(criteria))
	}

	return &(*bundles)[0], found, nil
}

func (c *Check) createCheck() (*api.CheckBundle, error) {
	// addr := c.agentAddress
	// if addr[0:1] == ":" {
	// 	addr = "localhost" + addr
	// }
	target := viper.GetString(config.KeyCheckTarget)
	if target == "" {
		return nil, errors.New("invalid check target (empty)")
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
			return nil, errors.Wrap(err, "selecting broker to create check")
		}

		brokerCID = broker.CID
	}

	if ok, _ := regexp.MatchString(`^[0-9]+$`, brokerCID); ok {
		brokerCID = "/broker/" + brokerCID
	}

	cfg.Brokers = []string{brokerCID}

	bundle, err := c.client.CreateCheckBundle(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "creating check bundle")
	}

	return bundle, nil
}
