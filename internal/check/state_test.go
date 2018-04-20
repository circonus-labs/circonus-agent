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

func TestLoadState(t *testing.T) {
	t.Log("Testing loadState")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("stateFile (empty)")
	{
		c := Check{stateFile: ""}

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
		c := Check{stateFile: "testdata/state/missing"}

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
		c := Check{stateFile: "testdata/state/bad.json"}

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
		c := Check{stateFile: "testdata/state/valid.json"}

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
		c := Check{stateFile: ""}

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
		c := Check{stateFile: "testdata/state/save.test"}

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
		viper.Set(config.KeyStateDir, "")
		c := Check{statePath: viper.GetString(config.KeyStateDir)}

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
		viper.Set(config.KeyStateDir, "testdata/state/missing")
		c := Check{statePath: viper.GetString(config.KeyStateDir)}

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
		viper.Set(config.KeyStateDir, "testdata/state/not_a_dir")
		c := Check{statePath: viper.GetString(config.KeyStateDir)}

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
		viper.Set(config.KeyStateDir, "testdata/state")
		c := Check{statePath: viper.GetString(config.KeyStateDir)}

		ok, err := c.verifyStatePath()
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		if !ok {
			t.Fatal("expected true")
		}
	}
}
