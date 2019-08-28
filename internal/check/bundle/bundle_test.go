// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package bundle

import (
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/gojuno/minimock/v3"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestFetchCheck(t *testing.T) {
	t.Log("Testing fetchCheck")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	viper.Reset()
	viper.Set(config.KeyAPITokenKey, "foo")
	viper.Set(config.KeyAPITokenApp, "bar")
	viper.Set(config.KeyAPIURL, "baz")

	mc := minimock.NewController(t)
	client := genMockClient(mc)
	c := Bundle{client: client}

	t.Log("cid (empty)")
	{
		cid := ""
		viper.Set(config.KeyCheckBundleID, cid)
		viper.Set(config.KeyCheckEnableNewMetrics, true)

		_, err := c.fetchCheckBundle(cid)
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != "invalid cid (empty)" {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}

	t.Log("cid (abc)")
	{
		cid := "abc"
		viper.Set(config.KeyCheckBundleID, cid)
		viper.Set(config.KeyCheckEnableNewMetrics, true)

		_, err := c.fetchCheckBundle(cid)
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != "invalid cid (abc)" {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}

	t.Log("api error")
	{
		cid := "000"
		viper.Set(config.KeyCheckBundleID, cid)
		viper.Set(config.KeyCheckEnableNewMetrics, true)

		_, err := c.fetchCheckBundle(cid)
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != "unable to retrieve check bundle (/check_bundle/000): forced mock api call error" {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}

	t.Log("valid")
	{
		cid := "1234"
		viper.Set(config.KeyCheckBundleID, cid)
		viper.Set(config.KeyCheckEnableNewMetrics, true)

		_, err := c.fetchCheckBundle(cid)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
	}
}

func TestFindCheck(t *testing.T) {
	t.Log("Testing findCheck")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	viper.Reset()
	viper.Set(config.KeyAPITokenKey, "foo")
	viper.Set(config.KeyAPITokenApp, "bar")
	viper.Set(config.KeyAPIURL, "baz")

	mc := minimock.NewController(t)
	client := genMockClient(mc)
	c := Bundle{client: client}

	t.Log("target (empty)")
	{

		target := ""
		viper.Set(config.KeyCheckTarget, target)
		viper.Set(config.KeyCheckEnableNewMetrics, true)

		_, found, err := c.findCheckBundle()
		if err == nil {
			t.Fatal("expected error")
		}
		if found != -1 {
			t.Fatal("expected found == -1")
		}

		if err.Error() != "invalid check bundle target (empty)" {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}

	t.Log("api error")
	{
		target := "000"
		viper.Set(config.KeyCheckTarget, target)
		viper.Set(config.KeyCheckEnableNewMetrics, true)

		_, found, err := c.findCheckBundle()
		if err == nil {
			t.Fatal("expected error")
		}
		if found != -1 {
			t.Fatal("expected found == -1")
		}

		if err.Error() != "searching for check bundle: forced mock api call error" {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}

	t.Log("not found")
	{
		target := "not_found"
		viper.Set(config.KeyCheckTarget, target)
		viper.Set(config.KeyCheckEnableNewMetrics, true)

		_, found, err := c.findCheckBundle()
		if err == nil {
			t.Fatal("expected error")
		}
		if found != 0 {
			t.Fatal("expected found == 0")
		}

		if err.Error() != `no check bundles matched criteria ((active:1)(type:"json:nad")(target:"not_found"))` {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}

	t.Log("multiple")
	{
		target := "multiple"
		viper.Set(config.KeyCheckTarget, target)
		viper.Set(config.KeyCheckEnableNewMetrics, true)

		_, found, err := c.findCheckBundle()
		if err == nil {
			t.Fatal("expected error")
		}
		if found != 2 {
			t.Fatal("expected found == 2")
		}

		if err.Error() != `more than one (2) check bundle matched criteria ((active:1)(type:"json:nad")(target:"multiple"))` {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}

	t.Log("valid")
	{
		target := "valid"
		viper.Set(config.KeyCheckTarget, target)
		viper.Set(config.KeyCheckEnableNewMetrics, true)

		_, found, err := c.findCheckBundle()
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		if found != 1 {
			t.Fatal("expected found == 1")
		}
	}
}

func TestCreateCheck(t *testing.T) {
	t.Log("Testing createCheck")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	viper.Reset()
	viper.Set(config.KeyAPITokenKey, "foo")
	viper.Set(config.KeyAPITokenApp, "bar")
	viper.Set(config.KeyAPIURL, "baz")

	mc := minimock.NewController(t)
	client := genMockClient(mc)
	c := Bundle{client: client}

	t.Log("target (empty)")
	{
		target := ""
		viper.Set(config.KeyCheckTarget, target)
		viper.Set(config.KeyCheckEnableNewMetrics, true)

		_, err := c.createCheckBundle()
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != "invalid check bundle target (empty)" {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}
}
