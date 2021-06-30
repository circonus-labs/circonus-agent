// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package builtins

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/rs/zerolog"
)

// fake collector stub

type foo struct {
	sync.Mutex
	lastEnd         time.Time
	lastStart       time.Time
	lastError       error
	lastMetrics     cgm.Metrics
	id              string
	logger          zerolog.Logger
	lastRunDuration time.Duration
}

func newFoo() collector.Collector {
	return &foo{id: "foo"}
}
func (f *foo) Collect(ctx context.Context) error {
	f.Lock()
	defer f.Unlock()
	f.lastStart = time.Now()
	f.lastMetrics = cgm.Metrics{"bar": cgm.Metric{Type: "i", Value: 1}}
	f.lastEnd = time.Now()
	f.lastRunDuration = time.Since(f.lastStart)
	return nil
}
func (f *foo) Flush() cgm.Metrics {
	f.Lock()
	defer f.Unlock()
	return f.lastMetrics
}
func (f *foo) ID() string {
	f.Lock()
	defer f.Unlock()
	return f.id
}
func (f *foo) Inventory() collector.InventoryStats {
	return collector.InventoryStats{
		ID:              f.id,
		LastRunStart:    f.lastStart.Format(time.RFC3339Nano),
		LastRunEnd:      f.lastEnd.Format(time.RFC3339Nano),
		LastRunDuration: f.lastRunDuration.String(),
		LastError:       f.lastError.Error(),
	}
}
func (f *foo) Logger() zerolog.Logger {
	return f.logger
}

// end fake collector stub

func TestNew(t *testing.T) {
	t.Log("Testing New")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	b, err := New(context.Background())
	if err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}
	if b == nil {
		t.Fatal("expected a builtins instance")
	}
}

func TestRun(t *testing.T) {
	t.Log("Testing Run")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("all (no collectors)")
	{
		b, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if b == nil {
			t.Fatal("expected a builtins instance")
		}

		rerr := b.Run(context.Background(), "")
		if rerr != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("w/id (no collectors)")
	{
		b, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if b == nil {
			t.Fatal("expected a builtins instance")
		}

		rerr := b.Run(context.Background(), "foo")
		if rerr != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("all (already running)")
	{
		b, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if b == nil {
			t.Fatal("expected a builtins instance")
			return
		}

		b.collectors["foo"] = newFoo()
		b.running = true

		rerr := b.Run(context.Background(), "")
		if rerr != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("w/id (unknown)")
	{
		b, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if b == nil {
			t.Fatal("expected a builtins instance")
			return
		}

		b.collectors["foo"] = newFoo()

		rerr := b.Run(context.Background(), "bar")
		if rerr != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("all (valid)")
	{
		b, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if b == nil {
			t.Fatal("expected a builtins instance")
			return
		}

		b.collectors["foo"] = newFoo()

		rerr := b.Run(context.Background(), "")
		if rerr != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("w/id (valid)")
	{
		b, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if b == nil {
			t.Fatal("expected a builtins instance")
			return
		}

		b.collectors["foo"] = newFoo()

		rerr := b.Run(context.Background(), "foo")
		if rerr != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}
}

func TestIsBuiltIn(t *testing.T) {
	t.Log("Testing IsBuiltIn")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("w/o id")
	{
		b, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if b == nil {
			t.Fatal("expected a builtins instance")
		}

		if b.IsBuiltin("") {
			t.Fatal("expected false")
		}
	}

	t.Log("w/id (not found)")
	{
		b, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if b == nil {
			t.Fatal("expected a builtins instance")
			return
		}

		if b.IsBuiltin("foo") {
			t.Fatal("expected false")
		}
	}

	t.Log("w/id (valid)")
	{
		b, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if b == nil {
			t.Fatal("expected a builtins instance")
			return
		}
		b.collectors["foo"] = newFoo()

		if !b.IsBuiltin("foo") {
			t.Fatal("expected true")
		}
	}
}

func TestFlush(t *testing.T) {
	t.Log("Testing Flush")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("w/o id")
	{
		b, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if b == nil {
			t.Fatal("expected a builtins instance")
		}

		metrics := b.Flush("")
		if metrics == nil { //nolint:staticcheck
			t.Fatal("expected metrics")
		}
		if len(*metrics) > 0 { //nolint:staticcheck
			t.Fatalf("expected empty metrics, got %#v", *metrics)
		}
	}

	t.Log("w/id (not found)")
	{
		b, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if b == nil {
			t.Fatal("expected a builtins instance")
		}

		metrics := b.Flush("foo")
		if metrics == nil { //nolint:staticcheck
			t.Fatal("expected metrics")
		}
		if len(*metrics) > 0 { //nolint:staticcheck
			t.Fatalf("expected empty metrics, got %#v", *metrics)
		}
	}

	t.Log("w/id (valid)")
	{
		b, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if b == nil {
			t.Fatal("expected a builtins instance")
		}

		b.collectors["foo"] = newFoo()
		_ = b.collectors["foo"].Collect(context.Background())

		metrics := b.Flush("foo")
		if metrics == nil { //nolint:staticcheck
			t.Fatal("expected metrics")
		}
		if len(*metrics) == 0 { //nolint:staticcheck
			t.Fatalf("expected at least 1 metric, got %#v", *metrics)
		}
	}
}
