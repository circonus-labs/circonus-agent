// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

//go:build !windows && !linux
// +build !windows,!linux

package builtins

import (
	"context"
	"fmt"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector/generic"
	appstats "github.com/maier/go-appstats"
	"github.com/rs/zerolog/log"
)

func (b *Builtins) configure(ctx context.Context) error {
	l := log.With().Str("pkg", "builtins").Logger()

	l.Debug().Msg("calling generic.New")
	collectors, err := generic.New()
	if err != nil {
		return fmt.Errorf("generic collectors: %w", err)
	}
	for _, c := range collectors {
		b.logger.Info().Str("id", c.ID()).Msg("enabled builtin")
		b.collectors[c.ID()] = c
		_ = appstats.IncrementInt("builtins.total")
	}

	return nil
}
