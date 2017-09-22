// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package plugins

import (
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

// NOTE: Scan is tested implicitly in main_test.go
//
// Setup details, some of the tests require that special ownership/permissions are set which are not 'git'able
//
// cd testdata
// ln -s symtest/testsym.sh
// ln -s symtest/invalid.sh
// mkdir noaccess ; chmod 700 noaccess && sudo chown root noaccess
// touch noaccesscfg.json && chmod 600 noaccesscfg.json && sudo chown root noaccesscfg.json

func TestScanPluginDirectory(t *testing.T) {
	t.Log("Testing scanPluginDirectory")

	p := New()
	p.active["purge_inactive"] = &plugin{
		ID:         "purge_inactive",
		Generation: 0,
	}

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("No plugin directory")
	{
		viper.Set(config.KeyPluginDir, "")

		expectErr := errors.Errorf("invalid plugin directory (none)")
		err := p.scanPluginDirectory()
		if err == nil {
			t.Fatal("expected error")
		}
		if expectErr.Error() != err.Error() {
			t.Fatalf("expected (%s) got (%s)", expectErr, err)
		}
	}

	t.Log("No access plugin directory")
	{
		dir := "testdata/noaccess"
		viper.Set(config.KeyPluginDir, dir)

		expectErr := errors.Errorf("open plugin directory: open %s: permission denied", dir)
		err := p.scanPluginDirectory()
		if err == nil {
			t.Fatalf("expected error (verify %s owned by root and mode 0700)", dir)
		}
		if expectErr.Error() != err.Error() {
			t.Fatalf("expected (%s) got (%s)", expectErr, err)
		}
	}

	t.Log("Valid plugin directory")
	{
		viper.Set(config.KeyPluginDir, "testdata/")

		err := p.scanPluginDirectory()
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}
}
