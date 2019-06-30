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
	"github.com/rs/zerolog"
)

func TestNewDiskCollector(t *testing.T) {
	t.Log("Testing NewDiskCollector")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("no config")
	{
		_, err := NewDiskCollector("", ProcFSPath)
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
		_, err := NewDiskCollector(filepath.Join("testdata", "missing"), ProcFSPath)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("config (bad syntax)")
	{
		_, err := NewDiskCollector(filepath.Join("testdata", "bad_syntax"), ProcFSPath)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (config no settings)")
	{
		c, err := NewDiskCollector(filepath.Join("testdata", "config_no_settings"), ProcFSPath)
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
		c, err := NewDiskCollector(filepath.Join("testdata", "config_id_setting"), ProcFSPath)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Disk).id != "foo" {
			t.Fatalf("expected foo, got (%s)", c.ID())
		}
	}

	t.Log("config (procfs path setting)")
	{
		c, err := NewDiskCollector(filepath.Join("testdata", "config_procfs_path_valid_setting"), ProcFSPath)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := "testdata"
		if c.(*Disk).procFSPath != expect {
			t.Fatalf("expected (%s), got (%s)", expect, c.(*Disk).procFSPath)
		}
	}

	t.Log("config (procfs path setting invalid)")
	{
		_, err := NewDiskCollector(filepath.Join("testdata", "config_procfs_path_invalid_setting"), ProcFSPath)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (include regex)")
	{
		c, err := NewDiskCollector(filepath.Join("testdata", "config_include_regex_valid_setting"), ProcFSPath)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := fmt.Sprintf(regexPat, `^foo`)
		if c.(*Disk).include.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, c.(*Disk).include.String())
		}
	}

	t.Log("config (include regex invalid)")
	{
		_, err := NewDiskCollector(filepath.Join("testdata", "config_include_regex_invalid_setting"), ProcFSPath)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (exclude regex)")
	{
		c, err := NewDiskCollector(filepath.Join("testdata", "config_exclude_regex_valid_setting"), ProcFSPath)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := fmt.Sprintf(regexPat, `^foo`)
		if c.(*Disk).exclude.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, c.(*Disk).exclude.String())
		}
	}

	t.Log("config (exclude regex invalid)")
	{
		_, err := NewDiskCollector(filepath.Join("testdata", "config_exclude_regex_invalid_setting"), ProcFSPath)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (run ttl 5m)")
	{
		c, err := NewDiskCollector(filepath.Join("testdata", "config_run_ttl_valid_setting"), ProcFSPath)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Disk).runTTL != 5*time.Minute {
			t.Fatal("expected 5m")
		}
	}

	t.Log("config (run ttl invalid)")
	{
		_, err := NewDiskCollector(filepath.Join("testdata", "config_run_ttl_invalid_setting"), ProcFSPath)
		if err == nil {
			t.Fatal("expected error")
		}
	}
}

func TestDiskFlush(t *testing.T) {
	t.Log("Testing Flush")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	c, err := NewDiskCollector(filepath.Join("testdata", "config_file_valid_setting"), ProcFSPath)
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
		c, err := NewDiskCollector(filepath.Join("testdata", "config_procfs_path_valid_setting"), ProcFSPath)
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
		c, err := NewDiskCollector(filepath.Join("testdata", "config_procfs_path_valid_setting"), ProcFSPath)
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
		c, err := NewDiskCollector(filepath.Join("testdata", "config_procfs_path_valid_setting"), ProcFSPath)
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
