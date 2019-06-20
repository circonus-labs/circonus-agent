// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package prometheus

import (
	"errors"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/rs/zerolog"
)

func TestFlush(t *testing.T) {
	t.Log("Testing Flush")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	c := &Prom{}

	metrics := c.Flush()
	if metrics == nil {
		t.Fatal("expected metrics")
	}
	if len(metrics) > 0 {
		t.Fatalf("expected empty metrics, got %v", metrics)
	}
}

func TestID(t *testing.T) {
	t.Log("Testing ID")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	c := &Prom{}

	expect := "promfetch"
	if c.ID() != expect {
		t.Fatalf("expected (%s) got (%s)", expect, c.ID())
	}
}

func TestInventory(t *testing.T) {
	t.Log("Testing Inventory")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	c := &Prom{}

	expect := "InventoryStats"
	inventory := c.Inventory()
	if it := reflect.TypeOf(inventory).Name(); it != expect {
		t.Fatalf("expected (%s) got (%s)", expect, it)
	}
}

func TestAddMetric(t *testing.T) {
	t.Log("Testing addMetric")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Testing invalid states/submissions")
	{
		c := &Prom{
			metricNameRegex: regexp.MustCompile("[\r\n\"']"),
		}
		if err := c.addMetric(nil, "", "", tags.Tags{}, "", ""); err == nil {
			t.Fatal("expected error")
		} else {
			expect := "invalid metric submission"
			if err.Error() != expect {
				t.Fatalf("expected (%s) got (%v)", expect, err)
			}
		}

		m := cgm.Metrics{}

		if err := c.addMetric(&m, "", "", tags.Tags{}, "", ""); err == nil {
			t.Fatalf("expected error")
		} else {
			expect := "invalid metric, no name"
			if err.Error() != expect {
				t.Fatalf("expected (%s) got (%v)", expect, err)
			}
		}

		if err := c.addMetric(&m, "", "foo", tags.Tags{}, "", ""); err == nil {
			t.Fatalf("expected error")
		} else {
			expect := "invalid metric, no type"
			if err.Error() != expect {
				t.Fatalf("expected (%s) got (%v)", expect, err)
			}
		}

		if err := c.addMetric(&m, "", "foo", tags.Tags{}, "t", ""); err == nil {
			t.Fatalf("expected error")
		} else {
			expect := "metric (foo) not active"
			if err.Error() != expect {
				t.Fatalf("expected (%s) got (%v)", expect, err)
			}
		}
	}

	t.Log("Testing valid states/submissions")
	{
		c := &Prom{
			metricNameRegex: regexp.MustCompile("[\r\n\"']"),
		}
		m := cgm.Metrics{}
		if err := c.addMetric(&m, "", "foo", tags.Tags{}, "t", ""); err != nil {
			t.Fatalf("expected no error, got (%v)", err)
		}
		if err := c.addMetric(&m, "bar", "baz", tags.Tags{}, "i", 10); err != nil {
			t.Fatalf("expected no error, got (%v)", err)
		}
	}
}

func TestSetStatus(t *testing.T) {
	t.Log("Testing setStatus")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	c := &Prom{}
	t.Log("\tno metrics, no error")
	c.setStatus(nil, nil)

	m := cgm.Metrics{}
	t.Log("\tmetrics, no error")
	c.setStatus(m, nil)
	t.Log("\tmetrics, error")
	c.setStatus(m, errors.New("foo"))

	t.Log("\tmetrics, no error, add last start")
	c.lastStart = time.Now()
	c.setStatus(m, nil)

}
