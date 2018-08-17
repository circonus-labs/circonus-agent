// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build !windows,!linux

package builtins

import (
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector/prometheus"
	appstats "github.com/maier/go-appstats"
)

func (b *Builtins) configure() error {
	prom, err := prometheus.New("")
	if err != nil {
		appstats.AddInt("builtins.total", 0)
		// appstats.MapAddInt("builtins", "total", 0)
		b.logger.Warn().Err(err).Msg("prom collector, disabling")
	} else {
		b.collectors[prom.ID()] = prom
		appstats.IncrementInt("builtins.total")
		// appstats.MapIncrementInt("builtins", "total")
	}
	return nil
}
