// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build linux

package procfs

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/rs/zerolog"
)

func TestNewLoadCollector(t *testing.T) {
	t.Log("Testing NewLoadCollector")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("no config")
	{
		_, err := NewLoadCollector("", defaults.HostProc)
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
		_, err := NewLoadCollector(filepath.Join("testdata", "missing"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("config (bad syntax)")
	{
		_, err := NewLoadCollector(filepath.Join("testdata", "bad_syntax"), defaults.HostProc)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (config no settings)")
	{
		c, err := NewLoadCollector(filepath.Join("testdata", "config_no_settings"), defaults.HostProc)
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
		c, err := NewLoadCollector(filepath.Join("testdata", "config_id_setting"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Load).id != "foo" {
			t.Fatalf("expected foo, got (%s)", c.ID())
		}
	}

	t.Log("config (procfs path setting)")
	{
		c, err := NewLoadCollector(filepath.Join("testdata", "config_procfs_path_valid_setting"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := "testdata"
		if c.(*Load).procFSPath != expect {
			t.Fatalf("expected (%s), got (%s)", expect, c.(*Load).procFSPath)
		}
	}

	t.Log("config (procfs path setting invalid)")
	{
		_, err := NewLoadCollector(filepath.Join("testdata", "config_procfs_path_invalid_setting"), defaults.HostProc)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (run ttl 5m)")
	{
		c, err := NewLoadCollector(filepath.Join("testdata", "config_run_ttl_valid_setting"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Load).runTTL != 5*time.Minute {
			t.Fatal("expected 5m")
		}
	}

	t.Log("config (run ttl invalid)")
	{
		_, err := NewLoadCollector(filepath.Join("testdata", "config_run_ttl_invalid_setting"), defaults.HostProc)
		if err == nil {
			t.Fatal("expected error")
		}
	}
}

func TestLoadFlush(t *testing.T) {
	t.Log("Testing Flush")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	c, err := NewLoadCollector(filepath.Join("testdata", "config_file_valid_setting"), defaults.HostProc)
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

func TestLoadCollect(t *testing.T) {
	t.Log("Testing Collect")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("already running")
	{
		c, err := NewLoadCollector(filepath.Join("testdata", "config_procfs_path_valid_setting"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}

		c.(*Load).running = true

		if err := c.Collect(context.Background()); err != nil {
			if err.Error() != collector.ErrAlreadyRunning.Error() {
				t.Fatalf("expected (%s) got (%s)", collector.ErrAlreadyRunning, err)
			}
		} else {
			t.Fatal("expected error")
		}
	}

	t.Log("ttl not expired")
	{
		c, err := NewLoadCollector(filepath.Join("testdata", "config_procfs_path_valid_setting"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}

		c.(*Load).runTTL = 60 * time.Second
		c.(*Load).lastEnd = time.Now()

		if err := c.Collect(context.Background()); err != nil {
			if err.Error() != collector.ErrTTLNotExpired.Error() {
				t.Fatalf("expected (%s) got (%s)", collector.ErrTTLNotExpired, err)
			}
		} else {
			t.Fatal("expected error")
		}
	}

	t.Log("good")
	{
		c, err := NewLoadCollector(filepath.Join("testdata", "config_procfs_path_valid_setting"), defaults.HostProc)
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
}
