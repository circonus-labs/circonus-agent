// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build linux

package procfs

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/rs/zerolog"
)

func TestNewCPUCollector(t *testing.T) {
	t.Log("Testing NewCPUCollector")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("no config")
	{
		_, err := NewCPUCollector("", defaults.HostProc)
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
		_, err := NewCPUCollector(filepath.Join("testdata", "missing"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("config (bad syntax)")
	{
		_, err := NewCPUCollector(filepath.Join("testdata", "bad_syntax"), defaults.HostProc)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (config no settings)")
	{
		c, err := NewCPUCollector(filepath.Join("testdata", "config_no_settings"), defaults.HostProc)
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
		c, err := NewCPUCollector(filepath.Join("testdata", "config_id_setting"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*CPU).id != "foo" {
			t.Fatalf("expected foo, got (%s)", c.ID())
		}
	}

	t.Log("config (procfs path setting)")
	{
		c, err := NewCPUCollector(filepath.Join("testdata", "config_procfs_path_valid_setting"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := "testdata"
		if c.(*CPU).procFSPath != expect {
			t.Fatalf("expected (%s), got (%s)", expect, c.(*CPU).procFSPath)
		}
	}

	t.Log("config (procfs path setting invalid)")
	{
		_, err := NewCPUCollector(filepath.Join("testdata", "config_procfs_path_invalid_setting"), defaults.HostProc)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (clock_hz setting)")
	{
		c, err := NewCPUCollector(filepath.Join("testdata", "config_clock_hz_valid_setting"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := float64(3)
		if c.(*CPU).clockNorm != expect {
			t.Fatalf("expected (%v), got (%v)", expect, c.(*CPU).clockNorm)
		}
	}

	t.Log("config (clock_hz setting invalid)")
	{
		_, err := NewCPUCollector(filepath.Join("testdata", "config_clock_hz_invalid_setting"), defaults.HostProc)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (report all cpus setting true)")
	{
		c, err := NewCPUCollector(filepath.Join("testdata", "config_report_all_cpus_true_setting"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if !c.(*CPU).reportAllCPUs {
			t.Fatal("expected true")
		}
	}

	t.Log("config (report all cpus setting false)")
	{
		c, err := NewCPUCollector(filepath.Join("testdata", "config_report_all_cpus_false_setting"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*CPU).reportAllCPUs {
			t.Fatal("expected false")
		}
	}

	t.Log("config (report all cpus setting invalid)")
	{
		_, err := NewCPUCollector(filepath.Join("testdata", "config_report_all_cpus_invalid_setting"), defaults.HostProc)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (run ttl 5m)")
	{
		c, err := NewCPUCollector(filepath.Join("testdata", "config_run_ttl_valid_setting"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*CPU).runTTL != 5*time.Minute {
			t.Fatal("expected 5m")
		}
	}

	t.Log("config (run ttl invalid)")
	{
		_, err := NewCPUCollector(filepath.Join("testdata", "config_run_ttl_invalid_setting"), defaults.HostProc)
		if err == nil {
			t.Fatal("expected error")
		}
	}
}

func TestCPUFlush(t *testing.T) {
	t.Log("Testing Flush")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	c, err := NewCPUCollector(filepath.Join("testdata", "config_file_valid_setting"), defaults.HostProc)
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
		c, err := NewCPUCollector(filepath.Join("testdata", "config_procfs_path_valid_setting"), defaults.HostProc)
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
		c, err := NewCPUCollector(filepath.Join("testdata", "config_procfs_path_valid_setting"), defaults.HostProc)
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
		c, err := NewCPUCollector(filepath.Join("testdata", "config_procfs_path_valid_setting"), defaults.HostProc)
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
