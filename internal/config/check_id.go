// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"encoding/json"
	"io/ioutil"
	"regexp"

	"github.com/pkg/errors"
)

func loadCOSICheckID(cfgFile string) (string, error) {
	data, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		return "", errors.Wrap(err, "Unable to access cosi check config")
	}

	var cfg cosiCheckConfig
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return "", errors.Wrapf(err, "Unable to parse cosi check cosi config (%s)", cfgFile)
	}
	if cfg.CID == "" {
		return "", errors.Errorf("Missing CID key, invalid cosi check config (%s)", cfgFile)
	}

	if err := validCheckID(cfg.CID); err != nil {
		return "", err
	}

	return cfg.CID, nil
}

func validCheckID(cid string) error {
	ok, err := regexp.MatchString("^(/check_bundle/)?[0-9]+$", cid)
	if err != nil {
		return errors.Wrapf(err, "Unable to verify Check ID (%s)", cid)
	}

	if !ok {
		return errors.Errorf("Invalid Check ID (%s)", cid)
	}

	return nil
}
