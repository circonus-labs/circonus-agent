// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

//go:build windows
// +build windows

package wmi

import (
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/spf13/viper"
)

func TestNew(t *testing.T) {
	t.Log("Testing New")

	viper.Set(config.KeyCollectors, []string{
		"wmi/memory",
		"wmi/processor",
	})

	c, err := New()
	if err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}
	if len(c) == 0 {
		t.Fatal("expected at least 1 collector.Collector")
	}
}
