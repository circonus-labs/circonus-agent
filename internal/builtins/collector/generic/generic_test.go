// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/spf13/viper"
)

func TestNew(t *testing.T) {
	t.Log("Testing New")

	viper.Set(config.KeyCollectors, []string{
		"generic/cpu",
		"generic/disk",
	})

	c, err := New()
	if err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}

	if len(c) == 0 {
		t.Fatal("expected at least 1 collector.Collector")
	}
}
