// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package procfs

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/rs/zerolog"
)

func TestNewCPUCollector(t *testing.T) {
	t.Log("Testing NewCPUCollector")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("no config")
	{
		_, err := NewCPUCollector("")
		if runtime.GOOS == "linux" {
			if err != nil {
				t.Fatalf("expected NO error, got (%s)", err)
			}
		} else {
			if err == nil {
				t.Fatal("expected error")
			}
		}
	}

	t.Log("config (missing)")
	{
		_, err := NewCPUCollector(filepath.Join("testdata", "missing"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("config (bad syntax)")
	{
		_, err := NewCPUCollector(filepath.Join("testdata", "bad_syntax"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (config no settings)")
	{
		c, err := NewCPUCollector(filepath.Join("testdata", "config_no_settings"))
		if runtime.GOOS == "linux" {
			if err != nil {
				t.Fatalf("expected NO error, got (%s)", err)
			}
			if c == nil {
				t.Fatal("expected no nil")
			}
		} else {
			if err == nil {
				t.Fatal("expected error")
			}
			if c != nil {
				t.Fatal("expected nil")
			}
		}
	}

	t.Log("config (id setting)")
	{
		c, err := NewCPUCollector(filepath.Join("testdata", "config_id_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*CPU).id != "foo" {
			t.Fatalf("expected foo, got (%s)", c.ID())
		}
	}

	t.Log("config (file setting)")
	{
		c, err := NewCPUCollector(filepath.Join("testdata", "config_file_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := "testdata/centos-7.2.stat"
		if c.(*CPU).file != expect {
			t.Fatalf("expected (%s), got (%s)", expect, c.(*CPU).file)
		}
	}

	t.Log("config (file setting invalid)")
	{
		_, err := NewCPUCollector(filepath.Join("testdata", "config_file_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (clock_hz setting)")
	{
		c, err := NewCPUCollector(filepath.Join("testdata", "config_clock_hz_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := float64(300)
		if c.(*CPU).clockHZ != expect {
			t.Fatalf("expected (%v), got (%v)", expect, c.(*CPU).clockHZ)
		}
	}

	t.Log("config (clock_hz setting invalid)")
	{
		_, err := NewCPUCollector(filepath.Join("testdata", "config_clock_hz_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (report all cpus setting true)")
	{
		c, err := NewCPUCollector(filepath.Join("testdata", "config_report_all_cpus_true_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if !c.(*CPU).reportAllCPUs {
			t.Fatal("expected true")
		}
	}

	t.Log("config (report all cpus setting false)")
	{
		c, err := NewCPUCollector(filepath.Join("testdata", "config_report_all_cpus_false_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*CPU).reportAllCPUs {
			t.Fatal("expected false")
		}
	}

	t.Log("config (report all cpus setting invalid)")
	{
		_, err := NewCPUCollector(filepath.Join("testdata", "config_report_all_cpus_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (metrics enabled setting)")
	{
		c, err := NewCPUCollector(filepath.Join("testdata", "config_metrics_enabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len(c.(*CPU).metricStatus) == 0 {
			t.Fatalf("expected >0 metric status settings, got (%#v)", c.(*CPU).metricStatus)
		}
		enabled, ok := c.(*CPU).metricStatus["foo"]
		if !ok {
			t.Fatalf("expected 'foo' key in metric status settings, got (%#v)", c.(*CPU).metricStatus)
		}
		if !enabled {
			t.Fatalf("expected 'foo' to be enabled in metric status settings, got (%#v)", c.(*CPU).metricStatus)
		}
	}

	t.Log("config (metrics disabled setting)")
	{
		c, err := NewCPUCollector(filepath.Join("testdata", "config_metrics_disabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len(c.(*CPU).metricStatus) == 0 {
			t.Fatalf("expected >0 metric status settings, got (%#v)", c.(*CPU).metricStatus)
		}
		enabled, ok := c.(*CPU).metricStatus["foo"]
		if !ok {
			t.Fatalf("expected 'foo' key in metric status settings, got (%#v)", c.(*CPU).metricStatus)
		}
		if enabled {
			t.Fatalf("expected 'foo' to be disabled in metric status settings, got (%#v)", c.(*CPU).metricStatus)
		}
	}

	t.Log("config (metrics default status enabled)")
	{
		c, err := NewCPUCollector(filepath.Join("testdata", "config_metrics_default_status_enabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if !c.(*CPU).metricDefaultActive {
			t.Fatal("expected true")
		}
	}

	t.Log("config (metrics default status disabled)")
	{
		c, err := NewCPUCollector(filepath.Join("testdata", "config_metrics_default_status_disabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*CPU).metricDefaultActive {
			t.Fatal("expected false")
		}
	}

	t.Log("config (metrics default status invalid)")
	{
		_, err := NewCPUCollector(filepath.Join("testdata", "config_metrics_default_status_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (run ttl 5m)")
	{
		c, err := NewCPUCollector(filepath.Join("testdata", "config_run_ttl_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*CPU).runTTL != 5*time.Minute {
			t.Fatal("expected 5m")
		}
	}

	t.Log("config (run ttl invalid)")
	{
		_, err := NewCPUCollector(filepath.Join("testdata", "config_run_ttl_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}
}

func TestCPUFlush(t *testing.T) {
	t.Log("Testing Flush")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	c, err := NewCPUCollector(filepath.Join("testdata", "config_file_valid_setting"))
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

func TestCPUCollect(t *testing.T) {
	t.Log("Testing Collect")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("already running")
	{
		c, err := NewCPUCollector(filepath.Join("testdata", "config_file_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}

		c.(*CPU).running = true

		if err := c.Collect(); err != nil {
			if err.Error() != collector.ErrAlreadyRunning.Error() {
				t.Fatalf("expected (%s) got (%s)", collector.ErrAlreadyRunning, err)
			}
		} else {
			t.Fatal("expected error")
		}
	}

	t.Log("ttl not expired")
	{
		c, err := NewCPUCollector(filepath.Join("testdata", "config_file_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}

		c.(*CPU).runTTL = 60 * time.Second
		c.(*CPU).lastEnd = time.Now()

		if err := c.Collect(); err != nil {
			if err.Error() != collector.ErrTTLNotExpired.Error() {
				t.Fatalf("expected (%s) got (%s)", collector.ErrTTLNotExpired, err)
			}
		} else {
			t.Fatal("expected error")
		}
	}

	t.Log("good")
	{
		c, err := NewCPUCollector(filepath.Join("testdata", "config_file_valid_setting"))
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
}
