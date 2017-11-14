// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package prom

import (
	"path"
	"testing"

	"github.com/rs/zerolog"
)

func TestNew(t *testing.T) {
	t.Log("Testing New")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("no config spec (force default)")
	{
		_, err := New("")
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("missing config file")
	{
		_, err := New(path.Join("testdata", "missing"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("empty config file")
	{
		_, err := New(path.Join("testdata", "empty"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("no prom urls")
	{
		_, err := New(path.Join("testdata", "no_urls"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("valid")
	{
		c, err := New(path.Join("testdata", "valid"))
		if err != nil {
			t.Fatal("expected NO error, got (%s)", err)
		}
		if len(c.(*Prom).urls) != 2 {
			t.Fatalf("expected 2 URLs, got (%#v)", c.(*Prom).urls)
		}
	}

}

func TestCollect(t *testing.T) {
	t.Log("Testing Collect")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	c, err := New(path.Join("testdata", "valid"))
	if err != nil {
		t.Fatal("expected NO error, got (%s)", err)
	}

	if err := c.Collect(); err != nil {
		t.Fatal("expected no error, got (%s)", err)
	}
}
