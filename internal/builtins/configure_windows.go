// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build windows

package builtins

import (
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector/windows/wmi"
	"github.com/rs/zerolog/log"
)

func (b *Builtins) configure() error {
	l := log.With().Str("pkg", "builtins").Logger()

	l.Debug().Msg("calling wmi.New")
	collectors, err := wmi.New()
	if err != nil {
		return err
	}
	l.Debug().Interface("collectors", collectors).Msg("loading collectors")
	for _, c := range collectors {
		b.collectors[c.ID()] = c
	}
	return nil
}
