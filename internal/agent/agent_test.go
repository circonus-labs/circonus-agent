// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package agent

import (
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestNew(t *testing.T) {
	t.Log("Testing New")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("No config")
	{
		_, err := New()
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("valid w/defaults")
	{
		viper.Set(config.KeyPluginDir, "testdata")
		viper.Set(config.KeyStatsdDisabled, true)
		a, err := New()
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
		if a == nil {
			t.Fatal("expected not nil")
		}
	}
}

func TestStart(t *testing.T) {
	t.Skip("not testing Start")
}

func TestStop(t *testing.T) {
	t.Log("Testing Stop")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("valid w/defaults")
	{
		viper.Set(config.KeyPluginDir, "testdata")
		viper.Set(config.KeyStatsdDisabled, true)
		a, err := New()
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
		if a == nil {
			t.Fatal("expected not nil")
		}

		a.Stop()
	}
}
