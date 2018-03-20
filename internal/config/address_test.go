// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
)

func TestParseListen(t *testing.T) {
	t.Log("Testing ParseListen")

	t.Log("empty spec")
	{
		spec := ""
		s, err := ParseListen(spec)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if s.String() != defaults.Listen {
			t.Fatalf("unexpected net spec (%s)", s.String())
		}
	}

	t.Log("port only (1234)")
	{
		spec := "1234"
		s, err := ParseListen(spec)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if s.String() != ":"+spec {
			t.Fatalf("unexpected net spec (%s)", s.String())
		}
	}

	t.Log("ipv4 only (127.0.0.1)")
	{
		spec := "127.0.0.1"
		s, err := ParseListen(spec)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if s.String() != spec+defaults.Listen {
			t.Fatalf("unexpected net spec (%s)", s.String())
		}
	}

	t.Log("ipv6 only ([::1])")
	{
		spec := "[::1]"
		s, err := ParseListen(spec)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if s.String() != spec+defaults.Listen {
			t.Fatalf("unexpected net spec (%s)", s.String())
		}
	}

	t.Log("invalid (::1)")
	{
		spec := "::1"
		_, err := ParseListen(spec)
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "parsing listen: address ::1: too many colons in address" {
			t.Fatalf("unexpected error (%v)", err)
		}
	}

	t.Log("invalid (127-0.0.1)")
	{
		spec := "127-0.0.1"
		_, err := ParseListen(spec)
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "resolving listen: lookup 127-0.0.1: no such host" {
			t.Fatalf("unexpected error (%v)", err)
		}
	}

	t.Log("invalid (127.0.0.1:abc)")
	{
		spec := "127.0.0.1:abc"
		_, err := ParseListen(spec)
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "resolving listen: lookup tcp/abc: nodename nor servname provided, or not known" {
			t.Fatalf("unexpected error (%v)", err)
		}
	}
}
