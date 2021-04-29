// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

var (
	errInvalidReverseCID = fmt.Errorf("invalid reverse check cid")
)

func validateReverseOptions() error {

	cid := viper.GetString(KeyCheckBundleID)

	// 1. cid = 'cosi' - try to load system check registration
	if strings.ToLower(cid) == cosiName {
		cosiCID, err := LoadCosiCheckID("system")
		if err != nil {
			return err
		}
		cid = cosiCID
		viper.Set(KeyCheckBundleID, cid)
		log.Debug().Str("cid", cid).Msg("reverse, cosi cid")
	}

	if cid != "" {
		// 2. explicit check bundle id
		// short form: just numeric id (e.g. --cid 123)
		// or, long form: with '/check_bundle/' prefix (e.g. --cid "/check_bundle/123")
		ok, err := IsValidCheckID(cid)
		if err != nil {
			return fmt.Errorf("reverse Check ID: %w", err)
		}
		if !ok {
			return fmt.Errorf("(%s): %w", cid, errInvalidReverseCID)
		}
		log.Debug().Str("cid", cid).Msg("reverse, specified cid")
	}

	// valid cid or, if cid empty, reverse will search for a cid
	return nil
}
