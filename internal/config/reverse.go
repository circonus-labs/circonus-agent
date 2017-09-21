// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"path/filepath"
	"strings"

	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func validateReverseOptions() error {

	cid := viper.GetString(KeyReverseCID)

	// 1. cid = 'cosi' - try to load system check registration
	if strings.ToLower(cid) == cosiName {
		cfgFile := filepath.Join(defaults.BasePath, "..", cosiName, "registration", "registration-check-system.json")
		cosiCID, err := loadCOSICheckID(cfgFile)
		if err != nil {
			return err
		}
		cid = cosiCID
		viper.Set(KeyReverseCID, cid)
		log.Debug().Str("cid", cid).Msg("reverse, cosi cid")
	}

	if cid != "" {
		// 2. explicit check bundle id
		// short form: just numeric id (e.g. --cid 123)
		// or, long form: with '/check_bundle/' prefix (e.g. --cid "/check_bundle/123")
		if err := validCheckID(cid); err != nil {
			return errors.Wrap(err, "Reverse Check ID")
		}
		log.Debug().Str("cid", cid).Msg("reverse, specified cid")
	}

	// valid cid or, if cid empty, reverse will search for a cid
	return nil
}
