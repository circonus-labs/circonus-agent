// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build windows

package wmi

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestNewMemoryCollector(t *testing.T) {
	t.Log("Testing NewMemoryCollector")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("no config")
	{
		_, err := NewMemoryCollector("")
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("config (missing)")
	{
		_, err := NewMemoryCollector(filepath.Join("testdata", "missing"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("config (bad syntax)")
	{
		_, err := NewMemoryCollector(filepath.Join("testdata", "bad_syntax"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (config no settings)")
	{
		c, err := NewMemoryCollector(filepath.Join("testdata", "config_no_settings"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c == nil {
			t.Fatal("expected no nil")
		}
	}

	t.Log("config (id setting)")
	{
		c, err := NewMemoryCollector(filepath.Join("testdata", "config_id_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Memory).id != "foo" {
			t.Fatalf("expected foo, got (%s)", c.ID())
		}
	}

	t.Log("config (metrics enabled setting)")
	{
		c, err := NewMemoryCollector(filepath.Join("testdata", "config_metrics_enabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len(c.(*Memory).metricStatus) == 0 {
			t.Fatalf("expected >0 metric status settings, got (%#v)", c.(*Memory).metricStatus)
		}
		enabled, ok := c.(*Memory).metricStatus["foo"]
		if !ok {
			t.Fatalf("expected 'foo' key in metric status settings, got (%#v)", c.(*Memory).metricStatus)
		}
		if !enabled {
			t.Fatalf("expected 'foo' to be enabled in metric status settings, got (%#v)", c.(*Memory).metricStatus)
		}
	}

	t.Log("config (metrics disabled setting)")
	{
		c, err := NewMemoryCollector(filepath.Join("testdata", "config_metrics_disabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len(c.(*Memory).metricStatus) == 0 {
			t.Fatalf("expected >0 metric status settings, got (%#v)", c.(*Memory).metricStatus)
		}
		enabled, ok := c.(*Memory).metricStatus["foo"]
		if !ok {
			t.Fatalf("expected 'foo' key in metric status settings, got (%#v)", c.(*Memory).metricStatus)
		}
		if enabled {
			t.Fatalf("expected 'foo' to be disabled in metric status settings, got (%#v)", c.(*Memory).metricStatus)
		}
	}

	t.Log("config (metrics default status enabled)")
	{
		c, err := NewMemoryCollector(filepath.Join("testdata", "config_metrics_default_status_enabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if !c.(*Memory).metricDefaultActive {
			t.Fatal("expected true")
		}
	}

	t.Log("config (metrics default status disabled)")
	{
		c, err := NewMemoryCollector(filepath.Join("testdata", "config_metrics_default_status_disabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Memory).metricDefaultActive {
			t.Fatal("expected false")
		}
	}

	t.Log("config (metrics default status invalid)")
	{
		_, err := NewMemoryCollector(filepath.Join("testdata", "config_metrics_default_status_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (run ttl 5m)")
	{
		c, err := NewMemoryCollector(filepath.Join("testdata", "config_run_ttl_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Memory).runTTL != 5*time.Minute {
			t.Fatal("expected 5m")
		}
	}

	t.Log("config (run ttl invalid)")
	{
		_, err := NewMemoryCollector(filepath.Join("testdata", "config_run_ttl_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}
}

func TestMemoryFlush(t *testing.T) {
	t.Log("Testing Flush")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	c, err := NewMemoryCollector("")
	if err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}

	metrics := c.Flush()
	if metrics == nil {
		t.Fatal("expected metrics")
	}
	if len(metrics) > 0 {
		t.Fatalf("expected empty metrics, got %v", metrics)
	}
}

func TestMemoryCollect(t *testing.T) {
	t.Log("Testing Collect")

	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	c, err := NewMemoryCollector("")
	if err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}

	if err := c.Collect(); err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}

	metrics := c.Flush()
	if metrics == nil {
		t.Fatal("expected error")
	}
	if len(metrics) == 0 {
		t.Fatalf("expected metrics, got %v", metrics)
	}
}
