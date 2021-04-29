// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package reverse handles reverse connections to brokers
package reverse

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/check"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/reverse/connection"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

type Reverse struct {
	agentAddress string
	configs      *check.ReverseConfigs
	chk          *check.Check
	logger       zerolog.Logger
	enabled      bool
}

func New(parentLogger zerolog.Logger, chk *check.Check, agentAddress string) (*Reverse, error) {
	if chk == nil {
		return nil, fmt.Errorf("invalid check (nil") //nolint:goerr113
	}
	if agentAddress == "" {
		return nil, fmt.Errorf("invalid agent address (empty)") //nolint:goerr113
	}

	r := &Reverse{
		agentAddress: agentAddress,
		chk:          chk,
		enabled:      viper.GetBool(config.KeyReverse),
	}

	if !r.enabled {
		return r, nil
	}

	cfgs, err := r.chk.GetReverseConfigs()
	if err != nil {
		return nil, fmt.Errorf("getting reverse configurations: %w", err)
	}
	r.configs = cfgs

	cm, err := chk.CheckMeta()
	if err != nil {
		return nil, fmt.Errorf("setting up reverse: %w", err)
	}
	r.logger = parentLogger.With().
		Str("pkg", "reverse").
		Str("bundle_cid", cm.BundleID).
		Str("check_cid", cm.CheckID).
		Str("check_uuid", cm.CheckUUID).
		Logger()

	return r, nil
}

// Start reverse connection(s) to the broker(s).
func (r *Reverse) Start(ctx context.Context) error {
	if !r.enabled {
		r.logger.Info().Msg("disabled, not starting")
		return nil
	}
	if r.configs == nil {
		return fmt.Errorf("invalid reverse configurations (nil)") //nolint:goerr113
	}
	if len(*r.configs) == 0 {
		return fmt.Errorf("invalid reverse configurations (zero)") //nolint:goerr113
	}

	lastRefresh := time.Now()
	refreshCheck := false
	rctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for {
		select {
		case <-rctx.Done():
			return nil
		default:
		}

		if time.Since(lastRefresh) > 5*time.Minute {
			refreshCheck = true
		}

		if refreshCheck {
			r.logger.Debug().Msg("refreshing check")
			if err := r.chk.RefreshReverseConfig(); err != nil {
				r.logger.Error().Err(err).Msg("refreshing reverse configuration")
				cancel()
				return fmt.Errorf("refresh reverse config: %w", err)
			}
			cfgs, err := r.chk.GetReverseConfigs()
			if err != nil {
				cancel()
				return fmt.Errorf("get reverse configurations: %w", err)
			}
			r.configs = cfgs
			refreshCheck = false
		}

		r.logger.Debug().Msg("find primary broker instance")
		primaryCN, err := r.chk.FindPrimaryBrokerInstance(rctx, r.configs)
		if err != nil {
			var nferr *check.ErrNoOwnerFound
			if errors.As(err, &nferr) {
				r.logger.Warn().Err(nferr).Msg("refreshing check bundle configuration")
				refreshCheck = true
				continue
			}
			// if nferr, ok := errors.Cause(err).(*check.ErrNoOwnerFound); ok { //nolint:errorlint
			// 	r.logger.Warn().Err(nferr).Msg("refreshing check bundle configuration")
			// 	refreshCheck = true
			// 	continue
			// }
			return fmt.Errorf("find primary broker: %w", err)
		}

		r.logger.Debug().Msg("set broker config")
		cfg, ok := (*r.configs)[primaryCN]
		if !ok {
			r.logger.Warn().Str("primary", primaryCN).Msg("primary broker cn not found, refreshing check")
			refreshCheck = true
			continue
		}

		r.logger.Debug().
			Str("broker", cfg.BrokerID).
			Str("cn", cfg.CN).
			Str("address", cfg.BrokerAddr.String()).
			Str("url", cfg.ReverseURL.String()).
			Msg("reverse broker config")
		rc, err := connection.New(r.logger, r.agentAddress, &cfg)
		if err != nil {
			cancel()
			return fmt.Errorf("new conn: %w", err)
		}

		var wg sync.WaitGroup

		wg.Add(1)

		go func() {
			r.logger.Debug().Msg("starting reverse connection")
			if err := rc.Start(rctx); err != nil {
				r.logger.Warn().Err(err).Msg("reverse connection")
				var cerr *connection.OpError
				if errors.As(err, &cerr) {
					if cerr.Fatal {
						cancel()
					} else if cerr.RefreshCheck {
						refreshCheck = true
					}
				}
				// otherwise, fall through and find the check owner again
			}
			wg.Done()
		}()

		wg.Wait()
	}
}
