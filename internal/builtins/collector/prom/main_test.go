// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package prom

import (
	"fmt"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestNew(t *testing.T) {
	t.Log("Testing New")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("no config spec (force default)")
	{
		_, err := New("")
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("missing config file")
	{
		_, err := New(path.Join("testdata", "missing"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("empty config file")
	{
		_, err := New(path.Join("testdata", "empty"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("no prom urls")
	{
		_, err := New(path.Join("testdata", "no_urls"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (include regex)")
	{
		c, err := New(filepath.Join("testdata", "config_include_regex_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := fmt.Sprintf(regexPat, `^foo`)
		if c.(*Prom).include.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, c.(*Prom).include.String())
		}
	}

	t.Log("config (include regex invalid)")
	{
		_, err := New(filepath.Join("testdata", "config_include_regex_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (exclude regex)")
	{
		c, err := New(filepath.Join("testdata", "config_exclude_regex_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := fmt.Sprintf(regexPat, `^foo`)
		if c.(*Prom).exclude.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, c.(*Prom).exclude.String())
		}
	}

	t.Log("config (exclude regex invalid)")
	{
		_, err := New(filepath.Join("testdata", "config_exclude_regex_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (metrics enabled setting)")
	{
		c, err := New(filepath.Join("testdata", "config_metrics_enabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len(c.(*Prom).metricStatus) == 0 {
			t.Fatalf("expected >0 metric status settings, got (%#v)", c.(*Prom).metricStatus)
		}
		enabled, ok := c.(*Prom).metricStatus["foo"]
		if !ok {
			t.Fatalf("expected 'foo' key in metric status settings, got (%#v)", c.(*Prom).metricStatus)
		}
		if !enabled {
			t.Fatalf("expected 'foo' to be enabled in metric status settings, got (%#v)", c.(*Prom).metricStatus)
		}
	}

	t.Log("config (metrics disabled setting)")
	{
		c, err := New(filepath.Join("testdata", "config_metrics_disabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len(c.(*Prom).metricStatus) == 0 {
			t.Fatalf("expected >0 metric status settings, got (%#v)", c.(*Prom).metricStatus)
		}
		enabled, ok := c.(*Prom).metricStatus["foo"]
		if !ok {
			t.Fatalf("expected 'foo' key in metric status settings, got (%#v)", c.(*Prom).metricStatus)
		}
		if enabled {
			t.Fatalf("expected 'foo' to be disabled in metric status settings, got (%#v)", c.(*Prom).metricStatus)
		}
	}

	t.Log("config (metrics default status enabled)")
	{
		c, err := New(filepath.Join("testdata", "config_metrics_default_status_enabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if !c.(*Prom).metricDefaultActive {
			t.Fatal("expected true")
		}
	}

	t.Log("config (metrics default status disabled)")
	{
		c, err := New(filepath.Join("testdata", "config_metrics_default_status_disabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Prom).metricDefaultActive {
			t.Fatal("expected false")
		}
	}

	t.Log("config (metrics default status invalid)")
	{
		_, err := New(filepath.Join("testdata", "config_metrics_default_status_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (run ttl 5m)")
	{
		c, err := New(filepath.Join("testdata", "config_run_ttl_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Prom).runTTL != 5*time.Minute {
			t.Fatal("expected 5m")
		}
	}

	t.Log("config (run ttl invalid)")
	{
		_, err := New(filepath.Join("testdata", "config_run_ttl_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("valid")
	{
		c, err := New(path.Join("testdata", "valid"))
		if err != nil {
			t.Fatal("expected NO error, got (%s)", err)
		}
		if len(c.(*Prom).urls) != 2 {
			t.Fatalf("expected 2 URLs, got (%#v)", c.(*Prom).urls)
		}
	}

}

func TestCollect(t *testing.T) {
	t.Log("Testing Collect")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	c, err := New(path.Join("testdata", "valid"))
	if err != nil {
		t.Fatal("expected NO error, got (%s)", err)
	}

	if err := c.Collect(); err != nil {
		t.Fatal("expected no error, got (%s)", err)
	}
}
