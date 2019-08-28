// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

//
// start actual tests for methods in main
//

func TestNew(t *testing.T) {
	t.Log("Testing New")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("check not needed")
	{
		viper.Reset()
		viper.Set(config.KeyCheckBundleID, "")
		viper.Set(config.KeyCheckCreate, false)
		viper.Set(config.KeyCheckEnableNewMetrics, false)
		viper.Set(config.KeyReverse, false)
		viper.Set(config.KeyAPITokenKey, "")
		viper.Set(config.KeyAPITokenApp, "")
		viper.Set(config.KeyAPIURL, "")

		_, err := New(nil)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}
}
