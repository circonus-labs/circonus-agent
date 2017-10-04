// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package appstats

import (
	"expvar"
	"sync"

	"github.com/pkg/errors"
)

var (
	once  sync.Once
	stats *expvar.Map
)

func init() {
	once.Do(func() {
		stats = expvar.NewMap("stats")
	})
}

// NewString creates a new string stat
func NewString(name string) error {
	if stats == nil {
		return errors.Errorf("stats not initialized")
	}

	if stats.Get(name) != nil {
		return errors.Errorf("stat (%s) already initialized", name)
	}

	stats.Set(name, new(expvar.String))

	return nil
}

// SetString sets string stat to value
func SetString(name string, value string) error {
	if stats == nil {
		return errors.Errorf("stats not initialized")
	}

	if stats.Get(name) == nil {
		NewString(name)
	}

	stats.Get(name).(*expvar.String).Set(value)

	return nil
}

// NewInt creates a new int stat
func NewInt(name string) error {
	if stats == nil {
		return errors.Errorf("stats not initialized")
	}

	if stats.Get(name) != nil {
		return errors.Errorf("stat (%s) already initialized", name)
	}

	stats.Set(name, new(expvar.Int))

	return nil
}

// SetInt sets int stat to value
func SetInt(name string, value int64) error {
	if stats == nil {
		return errors.Errorf("stats not initialized")
	}

	if stats.Get(name) == nil {
		NewInt(name)
	}

	stats.Get(name).(*expvar.Int).Set(value)

	return nil
}

// IncrementInt increment int stat
func IncrementInt(name string) error {
	if stats == nil {
		return errors.Errorf("stats not initialized")
	}

	if stats.Get(name) == nil {
		NewInt(name)
	}

	stats.Add(name, 1)

	return nil
}

// DecrementInt decrement int stat
func DecrementInt(name string) error {
	if stats == nil {
		return errors.Errorf("stats not initialized")
	}

	if stats.Get(name) == nil {
		NewInt(name)
	}

	stats.Add(name, -1)

	return nil
}

// NewFloat creates a new float stat
func NewFloat(name string) error {
	if stats == nil {
		return errors.Errorf("stats not initialized")
	}

	if stats.Get(name) != nil {
		return errors.Errorf("stat (%s) already initialized", name)
	}

	stats.Set(name, new(expvar.Float))

	return nil
}

// SetFloat sets a float stat to value
func SetFloat(name string, value float64) error {
	if stats == nil {
		return errors.Errorf("stats not initialized")
	}

	if stats.Get(name) == nil {
		NewFloat(name)
	}

	stats.Get(name).(*expvar.Float).Set(value)

	return nil
}

// AddFloat adds value to existing float
func AddFloat(name string, value float64) error {
	if stats == nil {
		return errors.Errorf("stats not initialized")
	}

	if stats.Get(name) == nil {
		NewFloat(name)
	}

	stats.Get(name).(*expvar.Float).Add(value)

	return nil
}
