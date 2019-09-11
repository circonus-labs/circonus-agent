// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package bundle

import (
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestLoadState(t *testing.T) {
	t.Log("Testing loadState")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("stateFile (empty)")
	{
		c := Bundle{stateFile: ""}

		_, err := c.loadState()
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "invalid state file (empty)" {
			t.Fatalf("unexpected error (%s)", err)
		}
	}

	t.Log("stateFile (missing)")
	{
		c := Bundle{stateFile: "testdata/state/missing"}

		_, err := c.loadState()
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "opening state file: open testdata/state/missing: no such file or directory" {
			t.Fatalf("unexpected error (%s)", err)
		}
	}

	t.Log("stateFile (bad)")
	{
		c := Bundle{stateFile: "testdata/state/bad.json"}

		_, err := c.loadState()
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "parsing state file: invalid character ':' after object key:value pair" {
			t.Fatalf("unexpected error (%s)", err)
		}
	}

	t.Log("stateFile (valid)")
	{
		c := Bundle{stateFile: "testdata/state/valid.json"}

		ms, err := c.loadState()
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		status, found := (*ms)["foo"]
		if !found {
			t.Fatalf("expected metric 'foo' in (%#v)", *ms)
		}
		if status != "active" {
			t.Fatalf("expected foo have status 'active' not (%s)", status)
		}
	}
}

func TestSaveState(t *testing.T) {
	t.Log("Testing saveState")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	ms := metricStates{"foo": "active"}

	t.Log("stateFile (empty)")
	{
		c := Bundle{stateFile: ""}

		err := c.saveState(&ms)
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "invalid state file (empty)" {
			t.Fatalf("unexpected error (%s)", err)
		}
	}

	t.Log("stateFile (valid)")
	{
		c := Bundle{stateFile: "testdata/state/save.test"}

		err := c.saveState(&ms)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
	}
}

func TestVerifyStatePath(t *testing.T) {
	t.Log("Testing verifyStatePath")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("statePath (empty)")
	{
		viper.Reset()
		viper.Set(config.KeyCheckMetricStateDir, "")
		c := Bundle{statePath: viper.GetString(config.KeyCheckMetricStateDir)}

		_, err := c.verifyStatePath()
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "invalid state path (empty)" {
			t.Fatalf("unexpected error (%s)", err)
		}
	}

	t.Log("statePath (missing)")
	{
		viper.Reset()
		viper.Set(config.KeyCheckMetricStateDir, "testdata/state/missing")
		c := Bundle{statePath: viper.GetString(config.KeyCheckMetricStateDir)}

		_, err := c.verifyStatePath()
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "stat state path: stat testdata/state/missing: no such file or directory" {
			t.Fatalf("unexpected error (%s)", err)
		}
	}

	t.Log("statePath (not directory)")
	{
		viper.Reset()
		viper.Set(config.KeyCheckMetricStateDir, "testdata/state/not_a_dir")
		c := Bundle{statePath: viper.GetString(config.KeyCheckMetricStateDir)}

		_, err := c.verifyStatePath()
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "state path is not a directory (testdata/state/not_a_dir)" {
			t.Fatalf("unexpected error (%s)", err)
		}
	}

	t.Log("statePath (valid)")
	{
		viper.Reset()
		viper.Set(config.KeyCheckMetricStateDir, "testdata/state")
		c := Bundle{statePath: viper.GetString(config.KeyCheckMetricStateDir)}

		ok, err := c.verifyStatePath()
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		if !ok {
			t.Fatal("expected true")
		}
	}
}
