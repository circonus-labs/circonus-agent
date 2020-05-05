// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package plugins

import (
	"context"
	"path"
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/builtins"
	"github.com/circonus-labs/circonus-agent/internal/config"
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

func TestScan(t *testing.T) {
	t.Log("Testing Scan")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("valid - plugin directory")
	{
		viper.Reset()
		viper.Set(config.KeyPluginDir, "testdata/")

		p, nerr := New(context.Background(), "")
		if nerr != nil {
			t.Fatalf("expected NO error, got (%s)", nerr)
		}
		b, berr := builtins.New(context.Background())
		if berr != nil {
			t.Fatalf("expected NO error, got (%s)", berr)
		}
		err := p.Scan(b)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("valid - plugin list")
	{
		viper.Reset()
		viper.Set(config.KeyPluginDir, "")
		viper.Set(config.KeyPluginList, []string{path.Join("testdata", "test.sh")})

		p, nerr := New(context.Background(), "")
		if nerr != nil {
			t.Fatalf("expected NO error, got (%s)", nerr)
		}
		b, berr := builtins.New(context.Background())
		if berr != nil {
			t.Fatalf("expected NO error, got (%s)", berr)
		}
		err := p.Scan(b)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}
}

func TestScanPluginDirectory(t *testing.T) {
	t.Log("Testing scanPluginDirectory")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	p, nerr := New(context.Background(), "")
	if nerr != nil {
		t.Fatalf("new err %s", nerr)
	}

	p.active["purge_inactive"] = &plugin{
		id: "purge_inactive",
	}

	b, berr := builtins.New(context.Background())
	if berr != nil {
		t.Fatalf("expected NO error, got (%s)", berr)
	}

	t.Log("No plugin directory")
	{
		p.pluginDir = ""
		err := p.scanPluginDirectory(b)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("No access plugin directory")
	{
		p.pluginDir = "testdata/noaccess"
		err := p.scanPluginDirectory(b)
		if err == nil {
			t.Fatalf("expected error (verify %s owned by root and mode 0700)", p.pluginDir)
		}
	}

	t.Log("Valid plugin directory")
	{
		p.pluginDir = "testdata/"
		err := p.scanPluginDirectory(b)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}
}
