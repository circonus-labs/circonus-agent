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

func TestNewDiskstatsCollector(t *testing.T) {
	t.Log("Testing NewDiskstatsCollector")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("no config")
	{
		_, err := NewDiskstatsCollector("", PROC_FS_PATH)
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
		_, err := NewDiskstatsCollector(filepath.Join("testdata", "missing"), PROC_FS_PATH)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("config (bad syntax)")
	{
		_, err := NewDiskstatsCollector(filepath.Join("testdata", "bad_syntax"), PROC_FS_PATH)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (config no settings)")
	{
		c, err := NewDiskstatsCollector(filepath.Join("testdata", "config_no_settings"), PROC_FS_PATH)
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
		c, err := NewDiskstatsCollector(filepath.Join("testdata", "config_id_setting"), PROC_FS_PATH)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Diskstats).id != "foo" {
			t.Fatalf("expected foo, got (%s)", c.ID())
		}
	}

	t.Log("config (procfs path setting)")
	{
		c, err := NewDiskstatsCollector(filepath.Join("testdata", "config_procfs_path_valid_setting"), PROC_FS_PATH)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := "testdata"
		if c.(*Diskstats).procFSPath != expect {
			t.Fatalf("expected (%s), got (%s)", expect, c.(*Diskstats).procFSPath)
		}
	}

	t.Log("config (procfs path setting invalid)")
	{
		_, err := NewDiskstatsCollector(filepath.Join("testdata", "config_procfs_path_invalid_setting"), PROC_FS_PATH)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (include regex)")
	{
		c, err := NewDiskstatsCollector(filepath.Join("testdata", "config_include_regex_valid_setting"), PROC_FS_PATH)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := fmt.Sprintf(regexPat, `^foo`)
		if c.(*Diskstats).include.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, c.(*Diskstats).include.String())
		}
	}

	t.Log("config (include regex invalid)")
	{
		_, err := NewDiskstatsCollector(filepath.Join("testdata", "config_include_regex_invalid_setting"), PROC_FS_PATH)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (exclude regex)")
	{
		c, err := NewDiskstatsCollector(filepath.Join("testdata", "config_exclude_regex_valid_setting"), PROC_FS_PATH)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := fmt.Sprintf(regexPat, `^foo`)
		if c.(*Diskstats).exclude.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, c.(*Diskstats).exclude.String())
		}
	}

	t.Log("config (exclude regex invalid)")
	{
		_, err := NewDiskstatsCollector(filepath.Join("testdata", "config_exclude_regex_invalid_setting"), PROC_FS_PATH)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (metrics enabled setting)")
	{
		c, err := NewDiskstatsCollector(filepath.Join("testdata", "config_metrics_enabled_setting"), PROC_FS_PATH)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len(c.(*Diskstats).metricStatus) == 0 {
			t.Fatalf("expected >0 metric status settings, got (%#v)", c.(*Diskstats).metricStatus)
		}
		enabled, ok := c.(*Diskstats).metricStatus["foo"]
		if !ok {
			t.Fatalf("expected 'foo' key in metric status settings, got (%#v)", c.(*Diskstats).metricStatus)
		}
		if !enabled {
			t.Fatalf("expected 'foo' to be enabled in metric status settings, got (%#v)", c.(*Diskstats).metricStatus)
		}
	}

	t.Log("config (metrics disabled setting)")
	{
		c, err := NewDiskstatsCollector(filepath.Join("testdata", "config_metrics_disabled_setting"), PROC_FS_PATH)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len(c.(*Diskstats).metricStatus) == 0 {
			t.Fatalf("expected >0 metric status settings, got (%#v)", c.(*Diskstats).metricStatus)
		}
		enabled, ok := c.(*Diskstats).metricStatus["foo"]
		if !ok {
			t.Fatalf("expected 'foo' key in metric status settings, got (%#v)", c.(*Diskstats).metricStatus)
		}
		if enabled {
			t.Fatalf("expected 'foo' to be disabled in metric status settings, got (%#v)", c.(*Diskstats).metricStatus)
		}
	}

	t.Log("config (metrics default status enabled)")
	{
		c, err := NewDiskstatsCollector(filepath.Join("testdata", "config_metrics_default_status_enabled_setting"), PROC_FS_PATH)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if !c.(*Diskstats).metricDefaultActive {
			t.Fatal("expected true")
		}
	}

	t.Log("config (metrics default status disabled)")
	{
		c, err := NewDiskstatsCollector(filepath.Join("testdata", "config_metrics_default_status_disabled_setting"), PROC_FS_PATH)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Diskstats).metricDefaultActive {
			t.Fatal("expected false")
		}
	}

	t.Log("config (metrics default status invalid)")
	{
		_, err := NewDiskstatsCollector(filepath.Join("testdata", "config_metrics_default_status_invalid_setting"), PROC_FS_PATH)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (run ttl 5m)")
	{
		c, err := NewDiskstatsCollector(filepath.Join("testdata", "config_run_ttl_valid_setting"), PROC_FS_PATH)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Diskstats).runTTL != 5*time.Minute {
			t.Fatal("expected 5m")
		}
	}

	t.Log("config (run ttl invalid)")
	{
		_, err := NewDiskstatsCollector(filepath.Join("testdata", "config_run_ttl_invalid_setting"), PROC_FS_PATH)
		if err == nil {
			t.Fatal("expected error")
		}
	}
}

func TestDiskstatsFlush(t *testing.T) {
	t.Log("Testing Flush")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	c, err := NewDiskstatsCollector(filepath.Join("testdata", "config_file_valid_setting"), PROC_FS_PATH)
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

func TestDiskstatsCollect(t *testing.T) {
	t.Log("Testing Collect")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("already running")
	{
		c, err := NewDiskstatsCollector(filepath.Join("testdata", "config_procfs_path_valid_setting"), PROC_FS_PATH)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}

		c.(*Diskstats).running = true

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
		c, err := NewDiskstatsCollector(filepath.Join("testdata", "config_procfs_path_valid_setting"), PROC_FS_PATH)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}

		c.(*Diskstats).runTTL = 60 * time.Second
		c.(*Diskstats).lastEnd = time.Now()

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
		c, err := NewDiskstatsCollector(filepath.Join("testdata", "config_procfs_path_valid_setting"), PROC_FS_PATH)
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
