// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/rs/zerolog"
)

func TestNewNetIFCollector(t *testing.T) {
	t.Log("Testing NewNetIFCollector")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	tests := []struct { //nolint:govet
		id          string
		cfgFile     string
		shouldFail  bool
		expectedErr string
	}{
		{"no config", "", true, "builtins.generic.if config: invalid config file (empty)"},
		{"missing config", filepath.Join("testdata", "missing"), false, ""},
		{"bad syntax", filepath.Join("testdata", "bad_syntax"), true, "builtins.generic.if config: parsing configuration file (testdata/bad_syntax.json): invalid character ',' looking for beginning of value"},
		{"no settings", filepath.Join("testdata", "config_no_settings"), false, ""},
	}

	for _, test := range tests {
		tst := test
		t.Run(tst.id, func(t *testing.T) {
			t.Parallel()
			_, err := NewNetIFCollector(tst.cfgFile, zerolog.Logger{})
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
		c, err := NewNetIFCollector(filepath.Join("testdata", "config_id_setting"), zerolog.Logger{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*IF).id != "foo" {
			t.Fatalf("expected foo, got (%s)", c.ID())
		}
	}

	t.Log("config (include regex setting - valid)")
	{
		c, err := NewNetIFCollector(filepath.Join("testdata", "config_include_regex_valid_setting"), zerolog.Logger{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*IF).include.String() != "^(?:^foo)$" {
			t.Fatal("expected (^(?:^foo)$)")
		}
	}

	t.Log("config (include regex setting - invalid)")
	{
		_, err := NewNetIFCollector(filepath.Join("testdata", "config_include_regex_invalid_setting"), zerolog.Logger{})
		if err.Error() != "builtins.generic.if compiling include regex: error parsing regexp: missing closing ]: `[foo)$`" {
			t.Fatalf("unexpected error, got (%s)", err)
		}
	}

	t.Log("config (exclude regex setting - valid)")
	{
		c, err := NewNetIFCollector(filepath.Join("testdata", "config_exclude_regex_valid_setting"), zerolog.Logger{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*IF).exclude.String() != "^(?:^foo)$" {
			t.Fatal("expected (^(?:^foo)$)")
		}
	}

	t.Log("config (exclude regex setting - invalid)")
	{
		_, err := NewNetIFCollector(filepath.Join("testdata", "config_exclude_regex_invalid_setting"), zerolog.Logger{})
		if err.Error() != "builtins.generic.if compiling exclude regex: error parsing regexp: missing closing ]: `[foo)$`" {
			t.Fatalf("unexpected error, got (%s)", err)
		}
	}

	t.Log("config (run ttl 5m)")
	{
		c, err := NewNetIFCollector(filepath.Join("testdata", "config_run_ttl_valid_setting"), zerolog.Logger{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*IF).runTTL != 5*time.Minute {
			t.Fatal("expected 5m")
		}
	}

	t.Log("config (run ttl invalid)")
	{
		_, err := NewNetIFCollector(filepath.Join("testdata", "config_run_ttl_invalid_setting"), zerolog.Logger{})
		if err == nil {
			t.Fatal("expected error")
		}
	}
}

func TestIFFlush(t *testing.T) {
	t.Log("Testing Flush")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	c, err := NewNetIFCollector(filepath.Join("testdata", "missing"), zerolog.Logger{})
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
		c, err := NewNetIFCollector(filepath.Join("testdata", "missing"), zerolog.Logger{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}

		c.(*IF).running = true

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
		c, err := NewNetIFCollector(filepath.Join("testdata", "missing"), zerolog.Logger{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}

		c.(*IF).runTTL = 60 * time.Second
		c.(*IF).lastEnd = time.Now()

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
		c, err := NewNetIFCollector(filepath.Join("testdata", "missing"), zerolog.Logger{})
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
