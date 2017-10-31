// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build windows

package wmi

import (
	"reflect"
	"testing"
)

func TestCollect(t *testing.T) {
	t.Log("Testing Collect")

	c := &wmicommon{
		id: "test",
	}

	err := c.Collect()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFlush(t *testing.T) {
	t.Log("Testing Flush")

	c := &wmicommon{
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

	c := &wmicommon{
		id: "test",
	}

	expect := "test"
	if c.ID() != expect {
		t.Fatalf("expected (%s) got (%s)", expect, c.ID())
	}
}

func TestInventory(t *testing.T) {
	t.Log("Testing Inventory")

	c := &wmicommon{
		id: "test",
	}

	expect := "InventoryStats"
	inventory := c.Inventory()
	if it := reflect.TypeOf(inventory).Name(); it != expect {
		t.Fatalf("expected (%s) got (%s)", expect, it)
	}
}
