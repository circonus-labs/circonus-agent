// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	"errors"
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestConfigure(t *testing.T) {
	t.Log("Testing configure")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("No settings")
	{
		_, _, err := configure()

		expectedErr := errors.New("reverse configuration (check): Initializing cgm API: API Token is required")
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("valid")
	{
		viper.Set(config.KeyReverseCID, "1234")
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "foo")
		viper.Set(config.KeyAPIURL, apiSim.URL)
		_, _, err := configure()
		viper.Set(config.KeyReverseCID, "")
		viper.Set(config.KeyAPITokenKey, "")
		viper.Set(config.KeyAPITokenApp, "")
		viper.Set(config.KeyAPIURL, "")

		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}
}
