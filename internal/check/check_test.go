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

func TestFetchCheck(t *testing.T) {
	t.Log("Testing fetchCheck")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("cid (empty)")
	{
		viper.Reset()
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "bar")
		viper.Set(config.KeyAPIURL, "baz")

		cid := ""
		viper.Set(config.KeyCheckBundleID, cid)
		viper.Set(config.KeyCheckEnableNewMetrics, true)

		c := Check{client: genMockClient()}

		_, err := c.fetchCheck(cid)
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != "invalid cid (empty)" {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}

	t.Log("cid (abc)")
	{
		viper.Reset()
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "bar")
		viper.Set(config.KeyAPIURL, "baz")

		cid := "abc"
		viper.Set(config.KeyCheckBundleID, cid)
		viper.Set(config.KeyCheckEnableNewMetrics, true)

		c := Check{client: genMockClient()}

		_, err := c.fetchCheck(cid)
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != "invalid cid (abc)" {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}

	t.Log("api error")
	{
		viper.Reset()
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "bar")
		viper.Set(config.KeyAPIURL, "baz")

		cid := "000"
		viper.Set(config.KeyCheckBundleID, cid)
		viper.Set(config.KeyCheckEnableNewMetrics, true)

		c := Check{client: genMockClient()}

		_, err := c.fetchCheck(cid)
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != "unable to retrieve check bundle (/check_bundle/000): forced mock api call error" {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}

	t.Log("valid")
	{
		viper.Reset()
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "bar")
		viper.Set(config.KeyAPIURL, "baz")

		cid := "1234"
		viper.Set(config.KeyCheckBundleID, cid)
		viper.Set(config.KeyCheckEnableNewMetrics, true)

		c := Check{client: genMockClient()}

		_, err := c.fetchCheck(cid)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
	}
}

func TestFindCheck(t *testing.T) {
	t.Log("Testing findCheck")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("target (empty)")
	{
		viper.Reset()
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "bar")
		viper.Set(config.KeyAPIURL, "baz")

		target := ""
		viper.Set(config.KeyCheckTarget, target)
		viper.Set(config.KeyCheckEnableNewMetrics, true)

		c := Check{client: genMockClient()}

		_, found, err := c.findCheck()
		if err == nil {
			t.Fatal("expected error")
		}
		if found != -1 {
			t.Fatal("expected found == -1")
		}

		if err.Error() != "invalid check target (empty)" {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}

	t.Log("api error")
	{
		viper.Reset()
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "bar")
		viper.Set(config.KeyAPIURL, "baz")

		target := "000"
		viper.Set(config.KeyCheckTarget, target)
		viper.Set(config.KeyCheckEnableNewMetrics, true)

		c := Check{client: genMockClient()}

		_, found, err := c.findCheck()
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
		viper.Reset()
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "bar")
		viper.Set(config.KeyAPIURL, "baz")

		target := "not_found"
		viper.Set(config.KeyCheckTarget, target)
		viper.Set(config.KeyCheckEnableNewMetrics, true)

		c := Check{client: genMockClient()}

		_, found, err := c.findCheck()
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
		viper.Reset()
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "bar")
		viper.Set(config.KeyAPIURL, "baz")

		target := "multiple"
		viper.Set(config.KeyCheckTarget, target)
		viper.Set(config.KeyCheckEnableNewMetrics, true)

		c := Check{client: genMockClient()}

		_, found, err := c.findCheck()
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
		viper.Reset()
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "bar")
		viper.Set(config.KeyAPIURL, "baz")

		target := "valid"
		viper.Set(config.KeyCheckTarget, target)
		viper.Set(config.KeyCheckEnableNewMetrics, true)

		c := Check{client: genMockClient()}

		_, found, err := c.findCheck()
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

	t.Log("target (empty)")
	{
		viper.Reset()
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "bar")
		viper.Set(config.KeyAPIURL, "baz")

		target := ""
		viper.Set(config.KeyCheckTarget, target)
		viper.Set(config.KeyCheckEnableNewMetrics, true)

		c := Check{client: genMockClient()}

		_, err := c.createCheck()
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != "invalid check target (empty)" {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}
}
