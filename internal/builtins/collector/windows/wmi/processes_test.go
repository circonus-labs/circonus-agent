// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build windows

package wmi

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestNewProcessesCollector(t *testing.T) {
	t.Log("Testing NewProcessesCollector")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("no config")
	{
		_, err := NewProcessesCollector("")
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("config (missing)")
	{
		_, err := NewProcessesCollector(filepath.Join("testdata", "missing"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("config (bad syntax)")
	{
		_, err := NewProcessesCollector(filepath.Join("testdata", "bad_syntax"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (config no settings)")
	{
		c, err := NewProcessesCollector(filepath.Join("testdata", "config_no_settings"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c == nil {
			t.Fatal("expected no nil")
		}
	}

	t.Log("config (include regex)")
	{
		c, err := NewProcessesCollector(filepath.Join("testdata", "config_include_regex_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := fmt.Sprintf(regexPat, `^foo`)
		if c.(*Processes).include.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, c.(*Processes).include.String())
		}
	}

	t.Log("config (include regex invalid)")
	{
		_, err := NewProcessesCollector(filepath.Join("testdata", "config_include_regex_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (exclude regex)")
	{
		c, err := NewProcessesCollector(filepath.Join("testdata", "config_exclude_regex_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := fmt.Sprintf(regexPat, `^foo`)
		if c.(*Processes).exclude.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, c.(*Processes).exclude.String())
		}
	}

	t.Log("config (exclude regex invalid)")
	{
		_, err := NewProcessesCollector(filepath.Join("testdata", "config_exclude_regex_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (id setting)")
	{
		c, err := NewProcessesCollector(filepath.Join("testdata", "config_id_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Processes).id != "foo" {
			t.Fatalf("expected foo, got (%s)", c.ID())
		}
	}

	t.Log("config (metrics enabled setting)")
	{
		c, err := NewProcessesCollector(filepath.Join("testdata", "config_metrics_enabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len(c.(*Processes).metricStatus) == 0 {
			t.Fatalf("expected >0 metric status settings, got (%#v)", c.(*Processes).metricStatus)
		}
		enabled, ok := c.(*Processes).metricStatus["foo"]
		if !ok {
			t.Fatalf("expected 'foo' key in metric status settings, got (%#v)", c.(*Processes).metricStatus)
		}
		if !enabled {
			t.Fatalf("expected 'foo' to be enabled in metric status settings, got (%#v)", c.(*Processes).metricStatus)
		}
	}

	t.Log("config (metrics disabled setting)")
	{
		c, err := NewProcessesCollector(filepath.Join("testdata", "config_metrics_disabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len(c.(*Processes).metricStatus) == 0 {
			t.Fatalf("expected >0 metric status settings, got (%#v)", c.(*Processes).metricStatus)
		}
		enabled, ok := c.(*Processes).metricStatus["foo"]
		if !ok {
			t.Fatalf("expected 'foo' key in metric status settings, got (%#v)", c.(*Processes).metricStatus)
		}
		if enabled {
			t.Fatalf("expected 'foo' to be disabled in metric status settings, got (%#v)", c.(*Processes).metricStatus)
		}
	}

	t.Log("config (metrics default status enabled)")
	{
		c, err := NewProcessesCollector(filepath.Join("testdata", "config_metrics_default_status_enabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if !c.(*Processes).metricDefaultActive {
			t.Fatal("expected true")
		}
	}

	t.Log("config (metrics default status disabled)")
	{
		c, err := NewProcessesCollector(filepath.Join("testdata", "config_metrics_default_status_disabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Processes).metricDefaultActive {
			t.Fatal("expected false")
		}
	}

	t.Log("config (metrics default status invalid)")
	{
		_, err := NewProcessesCollector(filepath.Join("testdata", "config_metrics_default_status_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (metric name regex)")
	{
		c, err := NewProcessesCollector(filepath.Join("testdata", "config_metric_name_regex_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := `^foo`
		if c.(*Processes).metricNameRegex.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, c.(*Processes).metricNameRegex.String())
		}
	}

	t.Log("config (metric name regex invalid)")
	{
		_, err := NewProcessesCollector(filepath.Join("testdata", "config_metric_name_regex_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (metric name char)")
	{
		c, err := NewProcessesCollector(filepath.Join("testdata", "config_metric_name_char_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Processes).metricNameChar != "-" {
			t.Fatal("expected '-'")
		}
	}

	t.Log("config (run ttl 5m)")
	{
		c, err := NewProcessesCollector(filepath.Join("testdata", "config_run_ttl_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Processes).runTTL != 5*time.Minute {
			t.Fatal("expected 5m")
		}
	}

	t.Log("config (run ttl invalid)")
	{
		_, err := NewProcessesCollector(filepath.Join("testdata", "config_run_ttl_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}
}

func TestProcessesFlush(t *testing.T) {
	t.Log("Testing Flush")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	c, err := NewProcessesCollector("")
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

func TestProcessesCollect(t *testing.T) {
	t.Log("Testing Collect")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	c, err := NewProcessesCollector("")
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
