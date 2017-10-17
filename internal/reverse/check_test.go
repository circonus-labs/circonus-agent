// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	"errors"
	"strings"
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestGetCheckConfig(t *testing.T) {
	t.Log("Testing getCheckConfig")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("No config")
	{
		viper.Set(config.KeyReverse, false)
		c, _ := New(defaults.Listen)
		_, _, err := c.getCheckConfig()

		expectedErr := errors.New("Initializing cgm API: API Token is required")
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("no matching check bundles")
	{
		viper.Set(config.KeyAPIURL, apiSim.URL)
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "foo")
		viper.Set(config.KeyReverse, false)
		c, _ := New(defaults.Listen)
		_, _, err := c.getCheckConfig()
		viper.Reset()

		expectedErr := `No check bundles matched criteria ((active:1)(type:"json:nad")(target:"`
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.HasPrefix(err.Error(), expectedErr) {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("multiple matching check bundles")
	{
		viper.Set(config.KeyReverseTarget, "multiple")
		viper.Set(config.KeyAPIURL, apiSim.URL)
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "foo")
		viper.Set(config.KeyReverse, false)
		c, _ := New(defaults.Listen)
		_, _, err := c.getCheckConfig()
		viper.Reset()

		expectedErr := errors.New(`More than one (2) check bundle matched criteria ((active:1)(type:"json:nad")(target:"multiple"))`)
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("bad check bundle id")
	{
		viper.Set(config.KeyReverseCID, "foo")
		viper.Set(config.KeyAPIURL, apiSim.URL)
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "foo")
		viper.Set(config.KeyReverse, false)
		c, _ := New(defaults.Listen)
		_, _, err := c.getCheckConfig()
		viper.Reset()

		expectedErr := errors.New("Invalid check bundle CID [foo]")
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("valid check bundle id")
	{
		viper.Set(config.KeyReverseCID, "1234")
		viper.Set(config.KeyAPIURL, apiSim.URL)
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "foo")
		viper.Set(config.KeyReverse, false)
		c, _ := New(defaults.Listen)
		_, _, err := c.getCheckConfig()
		viper.Reset()

		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("valid")
	{
		viper.Set(config.KeyReverseTarget, "test")
		viper.Set(config.KeyAPIURL, apiSim.URL)
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "foo")
		viper.Set(config.KeyReverse, false)
		c, _ := New(defaults.Listen)
		_, _, err := c.getCheckConfig()
		viper.Reset()

		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}
}
