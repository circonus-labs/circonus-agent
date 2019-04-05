// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/circonus-labs/go-apiclient"
	apiconf "github.com/circonus-labs/go-apiclient/config"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// initCheck initializes the check for the agent.
// 1. fetch a check explicitly provided via CID
// 2. search for a check matching the current system
// 3. create a check for the system if --check-create specified
// if fetched, found, or created - set Check.bundle
// otherwise, return an error
func (c *Check) initCheck(cid string, create bool) error {
	var bundle *apiclient.CheckBundle

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
			if !create || found != 0 {
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

	if bundle.Status != "active" {
		return errors.Errorf("check bundle (%s) is not active", bundle.CID)
	}

	c.bundle = bundle

	return nil
}

func (c *Check) fetchCheck(cid string) (*apiclient.CheckBundle, error) {
	if cid == "" {
		return nil, errors.New("invalid cid (empty)")
	}

	if ok, _ := regexp.MatchString(`^[0-9]+$`, cid); ok {
		cid = "/check_bundle/" + cid
	}

	if ok, _ := regexp.MatchString(`^/check_bundle/[0-9]+$`, cid); !ok {
		return nil, errors.Errorf("invalid cid (%s)", cid)
	}

	bundle, err := c.client.FetchCheckBundle(apiclient.CIDType(&cid))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to retrieve check bundle (%s)", cid)
	}

	return bundle, nil
}

func (c *Check) findCheck() (*apiclient.CheckBundle, int, error) {
	target := viper.GetString(config.KeyCheckTarget)
	if target == "" {
		return nil, -1, errors.New("invalid check target (empty)")
	}

	criteria := apiclient.SearchQueryType(fmt.Sprintf(`(active:1)(type:"json:nad")(target:"%s")`, target))
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

func (c *Check) createCheck() (*apiclient.CheckBundle, error) {

	// parse the first listen address to use as the required
	// URL in the check config
	var targetAddr string
	{
		serverList := viper.GetStringSlice(config.KeyListen)
		if len(serverList) == 0 {
			serverList = []string{defaults.Listen}
		}
		if serverList[0][0:1] == ":" {
			serverList[0] = "localhost" + serverList[0]
		}
		ta, err := config.ParseListen(serverList[0])
		if err != nil {
			c.logger.Error().Err(err).Str("addr", serverList[0]).Msg("resolving address")
			return nil, errors.Wrap(err, "parsing listen address")
		}
		targetAddr = ta.String()
	}

	target := viper.GetString(config.KeyCheckTarget)
	if target == "" {
		return nil, errors.New("invalid check target (empty)")
	}

	cfg := apiclient.NewCheckBundle()
	cfg.Target = target
	cfg.DisplayName = viper.GetString(config.KeyCheckTitle)
	if cfg.DisplayName == "" {
		cfg.DisplayName = cfg.Target + " /agent"
	}
	note := fmt.Sprintf("created by %s %s", release.NAME, release.VERSION)
	cfg.Notes = &note
	cfg.Type = "json:nad"
	cfg.Config = apiclient.CheckBundleConfig{apiconf.URL: "http://" + targetAddr + "/"}

	cfg.Metrics = []apiclient.CheckBundleMetric{}
	cfg.MetricFilters = defaults.CheckMetricFilters
	if viper.GetString(config.KeyCheckMetricFilters) != "" {
		var filters [][]string
		if err := json.Unmarshal([]byte(viper.GetString(config.KeyCheckMetricFilters)), &filters); err != nil {
			return nil, errors.Wrap(err, "parsing check metric filters")
		}
		cfg.MetricFilters = filters
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
