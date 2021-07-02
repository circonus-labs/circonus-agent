// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/rs/zerolog"
)

func TestNewNetProtoCollector(t *testing.T) {
	t.Log("Testing NewNetProtoCollector")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	tests := []struct {
		id          string
		cfgFile     string
		expectedErr string
		shouldFail  bool
	}{
		{"no config", "", "builtins.generic.proto config: invalid config file (empty)", true},
		{"missing config", filepath.Join("testdata", "missing"), "", false},
		{"bad syntax", filepath.Join("testdata", "bad_syntax"), "builtins.generic.proto config: parsing configuration file (testdata/bad_syntax.json): invalid character ',' looking for beginning of value", true},
		{"no settings", filepath.Join("testdata", "config_no_settings"), "", false},
	}

	for _, test := range tests {
		tst := test
		t.Run(tst.id, func(t *testing.T) {
			t.Parallel()
			_, err := NewNetProtoCollector(tst.cfgFile, zerolog.Logger{})
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
		c, err := NewNetProtoCollector(filepath.Join("testdata", "config_id_setting"), zerolog.Logger{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Proto).id != "foo" {
			t.Fatalf("expected foo, got (%s)", c.ID())
		}
	}

	t.Log("config (protocols setting)")
	{
		c, err := NewNetProtoCollector(filepath.Join("testdata", "config_protocols_setting"), zerolog.Logger{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len(c.(*Proto).protocols) != 2 {
			t.Fatal("expected 2")
		}
	}

	t.Log("config (run ttl 5m)")
	{
		c, err := NewNetProtoCollector(filepath.Join("testdata", "config_run_ttl_valid_setting"), zerolog.Logger{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Proto).runTTL != 5*time.Minute {
			t.Fatal("expected 5m")
		}
	}

	t.Log("config (run ttl invalid)")
	{
		_, err := NewNetProtoCollector(filepath.Join("testdata", "config_run_ttl_invalid_setting"), zerolog.Logger{})
		if err == nil {
			t.Fatal("expected error")
		}
	}
}

func TestProtoFlush(t *testing.T) {
	t.Log("Testing Flush")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	c, err := NewNetProtoCollector(filepath.Join("testdata", "missing"), zerolog.Logger{})
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

func TestProtoCollect(t *testing.T) {
	t.Log("Testing Collect")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("already running")
	{
		c, err := NewNetProtoCollector(filepath.Join("testdata", "missing"), zerolog.Logger{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}

		c.(*Proto).running = true

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
		c, err := NewNetProtoCollector(filepath.Join("testdata", "missing"), zerolog.Logger{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}

		c.(*Proto).runTTL = 60 * time.Second
		c.(*Proto).lastEnd = time.Now()

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
		c, err := NewNetProtoCollector(filepath.Join("testdata", "missing"), zerolog.Logger{})
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
		if runtime.GOOS == "darwin" {
			// NOTE: net protocols not implemented on darwin yet
			if len(metrics) != 0 {
				t.Fatalf("expected NO metrics, got %#v", metrics)
			}
		} else {
			if len(metrics) == 0 {
				t.Fatalf("expected metrics, got %#v", metrics)
			}
		}
	}
}
