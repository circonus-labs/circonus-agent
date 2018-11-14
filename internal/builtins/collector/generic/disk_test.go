// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/rs/zerolog"
)

func TestNewDiskCollector(t *testing.T) {
	t.Log("Testing NewDiskCollector")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	tests := []struct {
		id          string
		cfgFile     string
		shouldFail  bool
		expectedErr string
	}{
		{"no config", "", true, "builtins.generic.disk config: invalid config file (empty)"},
		{"missing config", filepath.Join("testdata", "missing"), false, ""},
		{"bad syntax", filepath.Join("testdata", "bad_syntax"), true, "builtins.generic.disk config: parsing configuration file (testdata/bad_syntax.json): invalid character ',' looking for beginning of value"},
		{"no settings", filepath.Join("testdata", "config_no_settings"), false, ""},
	}

	for _, test := range tests {
		tst := test
		t.Run(tst.id, func(t *testing.T) {
			t.Parallel()
			_, err := NewDiskCollector(tst.cfgFile)
			if tst.shouldFail {
				if err == nil {
					t.Fatalf("expected error")
				} else if err.Error() != tst.expectedErr {
					t.Fatalf("unexpected error (%s)", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error (%s)", err)
				}
			}
		})
	}

	t.Log("config (id setting)")
	{
		c, err := NewDiskCollector(filepath.Join("testdata", "config_id_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Disk).id != "foo" {
			t.Fatalf("expected foo, got (%s)", c.ID())
		}
	}

	t.Log("config (io devices setting)")
	{
		c, err := NewDiskCollector(filepath.Join("testdata", "config_io_devices_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len(c.(*Disk).ioDevices) != 2 {
			t.Fatal("expected 2")
		}
	}

	t.Log("config (metrics enabled setting)")
	{
		c, err := NewDiskCollector(filepath.Join("testdata", "config_metrics_enabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len(c.(*Disk).metricStatus) == 0 {
			t.Fatalf("expected >0 metric status settings, got (%#v)", c.(*Disk).metricStatus)
		}
		enabled, ok := c.(*Disk).metricStatus["foo"]
		if !ok {
			t.Fatalf("expected 'foo' key in metric status settings, got (%#v)", c.(*Disk).metricStatus)
		}
		if !enabled {
			t.Fatalf("expected 'foo' to be enabled in metric status settings, got (%#v)", c.(*Disk).metricStatus)
		}
	}

	t.Log("config (metrics disabled setting)")
	{
		c, err := NewDiskCollector(filepath.Join("testdata", "config_metrics_disabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len(c.(*Disk).metricStatus) == 0 {
			t.Fatalf("expected >0 metric status settings, got (%#v)", c.(*Disk).metricStatus)
		}
		enabled, ok := c.(*Disk).metricStatus["foo"]
		if !ok {
			t.Fatalf("expected 'foo' key in metric status settings, got (%#v)", c.(*Disk).metricStatus)
		}
		if enabled {
			t.Fatalf("expected 'foo' to be disabled in metric status settings, got (%#v)", c.(*Disk).metricStatus)
		}
	}

	t.Log("config (metrics default status enabled)")
	{
		c, err := NewDiskCollector(filepath.Join("testdata", "config_metrics_default_status_enabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if !c.(*Disk).metricDefaultActive {
			t.Fatal("expected true")
		}
	}

	t.Log("config (metrics default status disabled)")
	{
		c, err := NewDiskCollector(filepath.Join("testdata", "config_metrics_default_status_disabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Disk).metricDefaultActive {
			t.Fatal("expected false")
		}
	}

	t.Log("config (metrics default status invalid)")
	{
		_, err := NewDiskCollector(filepath.Join("testdata", "config_metrics_default_status_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (run ttl 5m)")
	{
		c, err := NewDiskCollector(filepath.Join("testdata", "config_run_ttl_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Disk).runTTL != 5*time.Minute {
			t.Fatal("expected 5m")
		}
	}

	t.Log("config (run ttl invalid)")
	{
		_, err := NewDiskCollector(filepath.Join("testdata", "config_run_ttl_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}
}

func TestDiskFlush(t *testing.T) {
	t.Log("Testing Flush")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	c, err := NewDiskCollector(filepath.Join("testdata", "missing"))
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

func TestDiskCollect(t *testing.T) {
	t.Log("Testing Collect")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("already running")
	{
		c, err := NewDiskCollector(filepath.Join("testdata", "missing"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}

		c.(*Disk).running = true

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
		c, err := NewDiskCollector(filepath.Join("testdata", "missing"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}

		c.(*Disk).runTTL = 60 * time.Second
		c.(*Disk).lastEnd = time.Now()

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
		c, err := NewDiskCollector(filepath.Join("testdata", "missing"))
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
