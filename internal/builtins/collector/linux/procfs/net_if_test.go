// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build linux

package procfs

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/rs/zerolog"
)

func TestNewIFCollector(t *testing.T) {
	t.Log("Testing NewIFCollector")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("no config")
	{
		_, err := NewNetIFCollector("", defaults.HostProc)
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
		_, err := NewNetIFCollector(filepath.Join("testdata", "missing"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("config (bad syntax)")
	{
		_, err := NewNetIFCollector(filepath.Join("testdata", "bad_syntax"), defaults.HostProc)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (config no settings)")
	{
		c, err := NewNetIFCollector(filepath.Join("testdata", "config_no_settings"), defaults.HostProc)
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
		c, err := NewNetIFCollector(filepath.Join("testdata", "config_id_setting"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*NetIF).id != "foo" {
			t.Fatalf("expected foo, got (%s)", c.ID())
		}
	}

	t.Log("config (procfs path setting)")
	{
		c, err := NewNetIFCollector(filepath.Join("testdata", "config_procfs_path_valid_setting"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := "testdata"
		if c.(*NetIF).procFSPath != expect {
			t.Fatalf("expected (%s), got (%s)", expect, c.(*NetIF).procFSPath)
		}
	}

	t.Log("config (procfs path setting invalid)")
	{
		_, err := NewNetIFCollector(filepath.Join("testdata", "config_procfs_path_invalid_setting"), defaults.HostProc)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (include regex)")
	{
		c, err := NewNetIFCollector(filepath.Join("testdata", "config_include_regex_valid_setting"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := fmt.Sprintf(regexPat, `^foo`)
		if c.(*NetIF).include.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, c.(*NetIF).include.String())
		}
	}

	t.Log("config (include regex invalid)")
	{
		_, err := NewNetIFCollector(filepath.Join("testdata", "config_include_regex_invalid_setting"), defaults.HostProc)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (exclude regex)")
	{
		c, err := NewNetIFCollector(filepath.Join("testdata", "config_exclude_regex_valid_setting"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := fmt.Sprintf(regexPat, `^foo`)
		if c.(*NetIF).exclude.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, c.(*NetIF).exclude.String())
		}
	}

	t.Log("config (exclude regex invalid)")
	{
		_, err := NewNetIFCollector(filepath.Join("testdata", "config_exclude_regex_invalid_setting"), defaults.HostProc)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (run ttl 5m)")
	{
		c, err := NewNetIFCollector(filepath.Join("testdata", "config_run_ttl_valid_setting"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*NetIF).runTTL != 5*time.Minute {
			t.Fatal("expected 5m")
		}
	}

	t.Log("config (run ttl invalid)")
	{
		_, err := NewNetIFCollector(filepath.Join("testdata", "config_run_ttl_invalid_setting"), defaults.HostProc)
		if err == nil {
			t.Fatal("expected error")
		}
	}
}

func TestIFFlush(t *testing.T) {
	t.Log("Testing Flush")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	c, err := NewNetIFCollector(filepath.Join("testdata", "config_file_valid_setting"), defaults.HostProc)
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

func TestIFCollect(t *testing.T) {
	t.Log("Testing Collect")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("already running")
	{
		c, err := NewNetIFCollector(filepath.Join("testdata", "config_procfs_path_valid_setting"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}

		c.(*NetIF).running = true

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
		c, err := NewNetIFCollector(filepath.Join("testdata", "config_procfs_path_valid_setting"), defaults.HostProc)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}

		c.(*NetIF).runTTL = 60 * time.Second
		c.(*NetIF).lastEnd = time.Now()

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
		c, err := NewNetIFCollector(filepath.Join("testdata", "config_procfs_path_valid_setting"), defaults.HostProc)
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
