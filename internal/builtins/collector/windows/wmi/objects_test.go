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

func TestNewObjectsCollector(t *testing.T) {
	t.Log("Testing NewObjectsCollector")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("no config")
	{
		_, err := NewObjectsCollector("")
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("config (missing)")
	{
		_, err := NewObjectsCollector(filepath.Join("testdata", "missing"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("config (bad syntax)")
	{
		_, err := NewObjectsCollector(filepath.Join("testdata", "bad_syntax"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (config no settings)")
	{
		c, err := NewObjectsCollector(filepath.Join("testdata", "config_no_settings"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c == nil {
			t.Fatal("expected no nil")
		}
	}

	t.Log("config (id setting)")
	{
		c, err := NewObjectsCollector(filepath.Join("testdata", "config_id_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Objects).id != "foo" {
			t.Fatalf("expected foo, got (%s)", c.ID())
		}
	}

	t.Log("config (metric name regex)")
	{
		c, err := NewObjectsCollector(filepath.Join("testdata", "config_metric_name_regex_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := `^foo`
		if c.(*Objects).metricNameRegex.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, c.(*Objects).metricNameRegex.String())
		}
	}

	t.Log("config (metric name regex invalid)")
	{
		_, err := NewObjectsCollector(filepath.Join("testdata", "config_metric_name_regex_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (metric name char)")
	{
		c, err := NewObjectsCollector(filepath.Join("testdata", "config_metric_name_char_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Objects).metricNameChar != "-" {
			t.Fatal("expected '-'")
		}
	}

	t.Log("config (run ttl 5m)")
	{
		c, err := NewObjectsCollector(filepath.Join("testdata", "config_run_ttl_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Objects).runTTL != 5*time.Minute {
			t.Fatal("expected 5m")
		}
	}

	t.Log("config (run ttl invalid)")
	{
		_, err := NewObjectsCollector(filepath.Join("testdata", "config_run_ttl_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}
}

func TestObjectsFlush(t *testing.T) {
	t.Log("Testing Flush")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	c, err := NewObjectsCollector("")
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

func TestObjectsCollect(t *testing.T) {
	t.Log("Testing Collect")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	c, err := NewObjectsCollector("")
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
