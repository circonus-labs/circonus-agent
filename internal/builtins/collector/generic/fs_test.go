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

func TestNewFSCollector(t *testing.T) {
	t.Log("Testing NewFSCollector")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	tests := []struct {
		id          string
		cfgFile     string
		shouldFail  bool
		expectedErr string
	}{
		{"no config", "", true, "builtins.generic.fs config: invalid config file (empty)"},
		{"missing config", filepath.Join("testdata", "missing"), false, ""},
		{"bad syntax", filepath.Join("testdata", "bad_syntax"), true, "builtins.generic.fs config: parsing configuration file (testdata/bad_syntax.json): invalid character ',' looking for beginning of value"},
		{"no settings", filepath.Join("testdata", "config_no_settings"), false, ""},
	}

	for _, test := range tests {
		tst := test
		t.Run(tst.id, func(t *testing.T) {
			t.Parallel()
			_, err := NewFSCollector(tst.cfgFile, zerolog.Logger{})
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
		c, err := NewFSCollector(filepath.Join("testdata", "config_id_setting"), zerolog.Logger{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*FS).id != "foo" {
			t.Fatalf("expected foo, got (%s)", c.ID())
		}
	}

	t.Log("config (include all devices setting true)")
	{
		c, err := NewFSCollector(filepath.Join("testdata", "config_include_all_devices_true_setting"), zerolog.Logger{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if !c.(*FS).allFSDevices {
			t.Fatal("expected true")
		}
	}

	t.Log("config (include all devices setting false)")
	{
		c, err := NewFSCollector(filepath.Join("testdata", "config_include_all_devices_false_setting"), zerolog.Logger{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*FS).allFSDevices {
			t.Fatal("expected false")
		}
	}

	t.Log("config (include all devices setting invalid)")
	{
		_, err := NewFSCollector(filepath.Join("testdata", "config_include_all_devices_invalid_setting"), zerolog.Logger{})
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (exclude fs types setting)")
	{
		c, err := NewFSCollector(filepath.Join("testdata", "config_exclude_fs_type_setting"), zerolog.Logger{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len(c.(*FS).excludeFSType) != 2 {
			t.Fatal("expected 2")
		}
	}

	t.Log("config (include fs regex setting - valid)")
	{
		c, err := NewFSCollector(filepath.Join("testdata", "config_include_fs_regex_valid_setting"), zerolog.Logger{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*FS).includeFS.String() != "^(?:^foo)$" {
			t.Fatal("expected (^(?:^foo)$)")
		}
	}

	t.Log("config (include fs regex setting - invalid)")
	{
		_, err := NewFSCollector(filepath.Join("testdata", "config_include_fs_regex_invalid_setting"), zerolog.Logger{})
		if err.Error() != "builtins.generic.fs compiling include FS regex: error parsing regexp: missing closing ]: `[foo)$`" {
			t.Fatalf("unexpected error, got (%s)", err)
		}
	}

	t.Log("config (exclude fs regex setting - valid)")
	{
		c, err := NewFSCollector(filepath.Join("testdata", "config_exclude_fs_regex_valid_setting"), zerolog.Logger{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*FS).excludeFS.String() != "^(?:^foo)$" {
			t.Fatal("expected (^(?:^foo)$)")
		}
	}

	t.Log("config (exclude fs regex setting - invalid)")
	{
		_, err := NewFSCollector(filepath.Join("testdata", "config_exclude_fs_regex_invalid_setting"), zerolog.Logger{})
		if err.Error() != "builtins.generic.fs compiling exclude FS regex: error parsing regexp: missing closing ]: `[foo)$`" {
			t.Fatalf("unexpected error, got (%s)", err)
		}
	}

	t.Log("config (run ttl 5m)")
	{
		c, err := NewFSCollector(filepath.Join("testdata", "config_run_ttl_valid_setting"), zerolog.Logger{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*FS).runTTL != 5*time.Minute {
			t.Fatal("expected 5m")
		}
	}

	t.Log("config (run ttl invalid)")
	{
		_, err := NewFSCollector(filepath.Join("testdata", "config_run_ttl_invalid_setting"), zerolog.Logger{})
		if err == nil {
			t.Fatal("expected error")
		}
	}
}

func TestFSFlush(t *testing.T) {
	t.Log("Testing Flush")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	c, err := NewFSCollector(filepath.Join("testdata", "missing"), zerolog.Logger{})
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

func TestFSCollect(t *testing.T) {
	t.Log("Testing Collect")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("already running")
	{
		c, err := NewFSCollector(filepath.Join("testdata", "missing"), zerolog.Logger{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}

		c.(*FS).running = true

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
		c, err := NewFSCollector(filepath.Join("testdata", "missing"), zerolog.Logger{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}

		c.(*FS).runTTL = 60 * time.Second
		c.(*FS).lastEnd = time.Now()

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
		c, err := NewFSCollector(filepath.Join("testdata", "missing"), zerolog.Logger{})
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
