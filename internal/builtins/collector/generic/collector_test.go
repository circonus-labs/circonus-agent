// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
)

func TestCollect(t *testing.T) {
	t.Log("Testing Collect")

	c := &gencommon{
		id: "test",
	}

	err := c.Collect(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFlush(t *testing.T) {
	t.Log("Testing Flush")

	c := &gencommon{
		id: "test",
	}

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

	c := &gencommon{
		id: "test",
	}

	expect := "test"
	if c.ID() != expect {
		t.Fatalf("expected (%s) got (%s)", expect, c.ID())
	}
}

func TestInventory(t *testing.T) {
	t.Log("Testing Inventory")

	c := &gencommon{
		id: "test",
	}

	expect := "InventoryStats"
	inventory := c.Inventory()
	if it := reflect.TypeOf(inventory).Name(); it != expect {
		t.Fatalf("expected (%s) got (%s)", expect, it)
	}
}

func TestAddMetric(t *testing.T) {
	t.Log("Testing addMetric")

	t.Log("Testing invalid states/submissions")
	{
		c := &gencommon{
			id: "foo",
		}
		if err := c.addMetric(nil, "", "", "", tags.Tags{}); err == nil {
			t.Fatal("expected error")
		} else {
			expect := "invalid metric, nil"
			if err.Error() != expect {
				t.Fatalf("expected (%s) got (%v)", expect, err)
			}
		}

		m := cgm.Metrics{}

		if err := c.addMetric(&m, "", "", "", tags.Tags{}); err == nil {
			t.Fatalf("expected error")
		} else {
			expect := "invalid metric, no name"
			if err.Error() != expect {
				t.Fatalf("expected (%s) got (%v)", expect, err)
			}
		}

		if err := c.addMetric(&m, "foo", "", "", tags.Tags{}); err == nil {
			t.Fatalf("expected error")
		} else {
			expect := "invalid metric, no type"
			if err.Error() != expect {
				t.Fatalf("expected (%s) got (%v)", expect, err)
			}
		}
	}

	t.Log("Testing valid submissions")
	{
		c := &gencommon{
			id: "foo",
		}
		m := cgm.Metrics{}
		if err := c.addMetric(&m, "foo", "t", "", tags.Tags{}); err != nil {
			t.Fatalf("expected no error, got (%v)", err)
		}
		if err := c.addMetric(&m, "baz", "i", 10, tags.Tags{tags.Tag{Category: "module", Value: "bar"}}); err != nil {
			t.Fatalf("expected no error, got (%v)", err)
		}
	}
}

func TestSetStatus(t *testing.T) {
	t.Log("Testing setStatus")

	c := &gencommon{
		id: "foo",
	}
	t.Log("\tno metrics, no error")
	c.setStatus(nil, nil)

	m := cgm.Metrics{}
	t.Log("\tmetrics, no error")
	c.setStatus(m, nil)
	t.Log("\tmetrics, error")
	c.setStatus(m, fmt.Errorf("foo")) //nolint:goerr113

	t.Log("\tmetrics, no error, add last start")
	c.lastStart = time.Now()
	c.setStatus(m, nil)

}
