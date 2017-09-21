// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func validateStatsdOptions() error {
	if viper.GetBool(KeyStatsdDisabled) {
		return nil
	}

	port := viper.GetString(KeyStatsdPort)
	if port == "" {
		return errors.New("Invalid StatsD port (empty)")
	}
	if ok, err := regexp.MatchString("^[0-9]+$", port); err != nil {
		return errors.Wrapf(err, "Invalid StatsD port (%s)", port)
	} else if !ok {
		return errors.Errorf("Invalid StatsD port (%s)", port)
	}
	if pnum, err := strconv.ParseUint(port, 10, 32); err != nil {
		return errors.Wrap(err, "Invalid StatsD port")
	} else if pnum < 1024 || pnum > 65535 {
		return errors.Errorf("Invalid StatsD port 1024>%s<65535", port)
	}

	// can be empty (all metrics go to host)
	// validate further if group check is enabled (see groupPrefix validation below)
	hostPrefix := viper.GetString(KeyStatsdHostPrefix)

	hostCat := viper.GetString(KeyStatsdHostCategory)
	if hostCat == "" {
		return errors.New("Invalid StatsD host category (empty)")
	}

	groupCID := viper.GetString(KeyStatsdGroupCID)
	if groupCID == "" {
		return nil // statsd group check support disabled, all metrics go to host
	}

	if groupCID == "cosi" {
		cfgFile := filepath.Join(defaults.BasePath, "..", cosiName, "registration", "registration-check-group.json")
		cid, err := loadCOSICheckID(cfgFile)
		if err != nil {
			return err
		}
		groupCID = cid
		viper.Set(KeyStatsdGroupCID, groupCID)
	}

	if err := validCheckID(groupCID); err != nil {
		return errors.Wrap(err, "StatsD Group Check ID")
	}

	groupPrefix := viper.GetString(KeyStatsdGroupPrefix)
	if hostPrefix == "" && groupPrefix == "" {
		return errors.New("StatsD host/group prefix mismatch (both empty)")
	}

	if hostPrefix == groupPrefix {
		return errors.New("StatsD host/group prefix mismatch (same)")
	}

	counterOp := viper.GetString(KeyStatsdGroupCounters)
	if counterOp == "" {
		return errors.New("Invalid StatsD counter operator (empty)")
	}
	if ok, err := regexp.MatchString("^(average|sum)$", counterOp); err != nil {
		return errors.Wrapf(err, "Invalid StatsD counter operator (%s)", counterOp)
	} else if !ok {
		return errors.Errorf("Invalid StatsD counter operator (%s)", counterOp)
	}

	gaugeOp := viper.GetString(KeyStatsdGroupGauges)
	if gaugeOp == "" {
		return errors.New("Invalid StatsD gauge operator (empty)")
	}
	if ok, err := regexp.MatchString("^(average|sum)$", gaugeOp); err != nil {
		return errors.Wrapf(err, "Invalid StatsD gauge operator (%s)", gaugeOp)
	} else if !ok {
		return errors.Errorf("Invalid StatsD gauge operator (%s)", gaugeOp)
	}

	setOp := viper.GetString(KeyStatsdGroupSets)
	if setOp == "" {
		return errors.New("Invalid StatsD set operator (empty)")
	}
	if ok, err := regexp.MatchString("^(average|sum)$", setOp); err != nil {
		return errors.Wrapf(err, "Invalid StatsD set operator (%s)", setOp)
	} else if !ok {
		return errors.Errorf("Invalid StatsD set operator (%s)", setOp)
	}

	return nil
}
