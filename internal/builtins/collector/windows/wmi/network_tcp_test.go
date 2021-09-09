// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

//go:build windows
// +build windows

package wmi

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestNewNetTCPCollector(t *testing.T) {
	t.Log("Testing NewNetTCPCollector")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("no config")
	{
		_, err := NewNetTCPCollector("")
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("config (missing)")
	{
		_, err := NewNetTCPCollector(filepath.Join("testdata", "missing"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("config (bad syntax)")
	{
		_, err := NewNetTCPCollector(filepath.Join("testdata", "bad_syntax"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (config no settings)")
	{
		c, err := NewNetTCPCollector(filepath.Join("testdata", "config_no_settings"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c == nil {
			t.Fatal("expected no nil")
		}
	}

	t.Log("config (enable ipv4 setting true)")
	{
		c, err := NewNetTCPCollector(filepath.Join("testdata", "config_enable_ipv4_true_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if !c.(*NetTCP).ipv4Enabled {
			t.Fatal("expected true")
		}
	}

	t.Log("config (enable ipv4 setting false)")
	{
		c, err := NewNetTCPCollector(filepath.Join("testdata", "config_enable_ipv4_false_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*NetTCP).ipv4Enabled {
			t.Fatal("expected false")
		}
	}

	t.Log("config (enable ipv4 setting invalid)")
	{
		_, err := NewNetTCPCollector(filepath.Join("testdata", "config_enable_ipv4_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (enable ipv6 setting true)")
	{
		c, err := NewNetTCPCollector(filepath.Join("testdata", "config_enable_ipv6_true_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if !c.(*NetTCP).ipv6Enabled {
			t.Fatal("expected true")
		}
	}

	t.Log("config (enable ipv6 setting false)")
	{
		c, err := NewNetTCPCollector(filepath.Join("testdata", "config_enable_ipv6_false_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*NetTCP).ipv6Enabled {
			t.Fatal("expected false")
		}
	}

	t.Log("config (enable ipv6 setting invalid)")
	{
		_, err := NewNetTCPCollector(filepath.Join("testdata", "config_enable_ipv6_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (id setting)")
	{
		c, err := NewNetTCPCollector(filepath.Join("testdata", "config_id_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*NetTCP).id != "foo" {
			t.Fatalf("expected foo, got (%s)", c.ID())
		}
	}

	t.Log("config (metric name regex)")
	{
		c, err := NewNetTCPCollector(filepath.Join("testdata", "config_metric_name_regex_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := `^foo`
		if c.(*NetTCP).metricNameRegex.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, c.(*NetTCP).metricNameRegex.String())
		}
	}

	t.Log("config (metric name regex invalid)")
	{
		_, err := NewNetTCPCollector(filepath.Join("testdata", "config_metric_name_regex_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (metric name char)")
	{
		c, err := NewNetTCPCollector(filepath.Join("testdata", "config_metric_name_char_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*NetTCP).metricNameChar != "-" {
			t.Fatal("expected '-'")
		}
	}

	t.Log("config (run ttl 5m)")
	{
		c, err := NewNetTCPCollector(filepath.Join("testdata", "config_run_ttl_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*NetTCP).runTTL != 5*time.Minute {
			t.Fatal("expected 5m")
		}
	}

	t.Log("config (run ttl invalid)")
	{
		_, err := NewNetTCPCollector(filepath.Join("testdata", "config_run_ttl_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}
}

func TestNetTCPFlush(t *testing.T) {
	t.Log("Testing Flush")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	c, err := NewNetTCPCollector("")
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

func TestNetTCPCollect(t *testing.T) {
	t.Log("Testing Collect")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	c, err := NewNetTCPCollector("")
	if err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}

	if err := c.Collect(context.Background()); err != nil {
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
