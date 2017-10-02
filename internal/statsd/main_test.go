// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package statsd

import (
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestNew(t *testing.T) {
	t.Log("Testing New")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Disabled")
	{
		viper.Set(config.KeyStatsdDisabled, true)
		s, err := New()
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if s == nil {
			t.Fatal("expected not nil")
		}
		viper.Reset()
	}

	t.Log("Enabled")
	{
		viper.Set(config.KeyStatsdDisabled, false)
		s, err := New()
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if s == nil {
			t.Fatal("expected not nil")
		}
		viper.Reset()
	}
}

func TestStart(t *testing.T) {
	t.Log("Testing Start")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Disabled")
	{
		viper.Set(config.KeyStatsdDisabled, true)
		s, err := New()
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		s.Start()
		viper.Reset()
	}

	t.Log("Enabled w/Stop")
	{
		viper.Set(config.KeyStatsdDisabled, false)
		s, err := New()
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if s == nil {
			t.Fatal("expected not nil")
		}
		time.AfterFunc(2*time.Second, func() {
			s.Stop()
		})
		s.Start()
		viper.Reset()
	}

	t.Log("Enabled w/direct server close")
	{
		viper.Set(config.KeyStatsdDisabled, false)
		s, err := New()
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if s == nil {
			t.Fatal("expected not nil")
		}
		time.AfterFunc(2*time.Second, func() {
			s.listener.Close()
		})
		s.Start()
		viper.Reset()
	}
}

func TestStop(t *testing.T) {
	t.Log("Testing Stop")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Disabled")
	{
		viper.Set(config.KeyStatsdDisabled, true)
		s, err := New()
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		s.Stop()
		viper.Reset()
	}

	t.Log("Enabled")
	{
		viper.Set(config.KeyStatsdDisabled, false)
		s, err := New()
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if s == nil {
			t.Fatal("expected not nil")
		}
		time.AfterFunc(2*time.Second, func() {
			s.Stop()
		})
		s.Start()
		viper.Reset()
	}
}

func TestFlush(t *testing.T) {
	t.Log("Testing Flush")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Flush (disabled)")
	{
		viper.Set(config.KeyStatsdDisabled, true)
		s, err := New()
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		metrics := s.Flush()
		viper.Reset()

		if metrics != nil {
			t.Fatalf("expected nil, got (%#v)", metrics)
		}
	}

	t.Log("Flush (no stats)")
	{
		viper.Set(config.KeyStatsdDisabled, false)
		s, err := New()
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		metrics := s.Flush()
		viper.Reset()

		if metrics == nil {
			t.Fatal("expected not nil")
		}
		if len(*metrics) != 0 {
			t.Fatalf("expected empty metrics, got (%#v)", metrics)
		}
	}

	t.Log("Flush (no stats, nil hostMetrics)")
	{
		viper.Set(config.KeyStatsdDisabled, false)
		s, err := New()
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		s.hostMetrics = nil
		metrics := s.Flush()
		viper.Reset()

		if metrics == nil {
			t.Fatal("expected not nil")
		}
		if len(*metrics) != 0 {
			t.Fatalf("expected empty metrics, got (%#v)", metrics)
		}
	}
}
