// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package procfs

import (
	"runtime"
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/spf13/viper"
)

func TestNew(t *testing.T) {
	t.Log("Testing New")

	viper.Set(config.KeyCollectors, []string{
		"cpu",
		"disk",
	})

	c, err := New()
	if err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}

	if runtime.GOOS == "linux" {
		if len(c) == 0 {
			t.Fatal("expected at least 1 collector.Collector")
		}
	} else {
		if len(c) != 0 {
			t.Fatal("expected 0 collectors")
		}
	}
}
