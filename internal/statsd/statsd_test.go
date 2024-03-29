// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package statsd

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestNew(t *testing.T) {
	t.Log("Testing New")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Disabled")
	{
		viper.Set(config.KeyStatsdDisabled, true)
		s, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if s == nil {
			t.Fatal("expected not nil")
		}
		viper.Reset()
	}

	t.Log("Enabled - no port")
	{
		viper.Set(config.KeyStatsdDisabled, false)
		expect := fmt.Errorf("invalid StatsD port (empty)") //nolint:goerr113
		_, err := New(context.Background())
		if err == nil {
			t.Fatal("expect error")
		}
		if err.Error() != expect.Error() {
			t.Fatalf("expected (%s) got (%s)", expect, err)
		}
		viper.Reset()
	}

	t.Log("Enabled - port 65125, invalid host category")
	{
		viper.Set(config.KeyStatsdDisabled, false)
		viper.Set(config.KeyStatsdPort, "65125")
		expect := fmt.Errorf("invalid StatsD host category (empty)") //nolint:goerr113
		_, err := New(context.Background())
		if err == nil {
			t.Fatal("expect error")
		}
		if err.Error() != expect.Error() {
			t.Fatalf("expected (%s) got (%s)", expect, err)
		}
		viper.Reset()
	}

	t.Log("Enabled - port 65125, default host category")
	{
		viper.Set(config.KeyStatsdDisabled, false)
		viper.Set(config.KeyStatsdPort, "65125")
		viper.Set(config.KeyStatsdHostCategory, defaults.StatsdHostCategory)
		_, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
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
		s, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		err = s.Start()
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		viper.Reset()
	}

	t.Log("Enabled w/context cancel")
	{
		viper.Set(config.KeyStatsdDisabled, false)
		viper.Set(config.KeyStatsdPort, "65125")
		viper.Set(config.KeyStatsdHostCategory, defaults.StatsdHostCategory)
		ctx, cancel := context.WithCancel(context.Background())
		s, err := New(ctx)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if s == nil {
			t.Fatal("expected not nil")
		}
		time.AfterFunc(2*time.Second, func() {
			cancel()
		})

		if err := s.Start(); err != nil {
			t.Fatalf("unexpected error (%s)", err)
		}
		viper.Reset()
	}
}

func TestFlush(t *testing.T) {
	t.Log("Testing Flush")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Flush (disabled)")
	{
		viper.Set(config.KeyStatsdDisabled, true)
		s, err := New(context.Background())
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
		viper.Set(config.KeyStatsdPort, "65125")
		viper.Set(config.KeyStatsdHostCategory, defaults.StatsdHostCategory)
		s, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		metrics := s.Flush()
		if metrics == nil {
			t.Fatal("expected not nil")
			return
		}
		if len(*metrics) != 0 {
			t.Fatalf("expected empty metrics, got (%#v)", metrics)
		}
		viper.Reset()
	}

	t.Log("Flush (no stats, nil hostMetrics)")
	{
		viper.Set(config.KeyStatsdDisabled, false)
		viper.Set(config.KeyStatsdPort, "65125")
		viper.Set(config.KeyStatsdHostCategory, defaults.StatsdHostCategory)
		s, err := New(context.Background())
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		s.hostMetrics = nil
		metrics := s.Flush()
		if metrics == nil {
			t.Fatal("expected not nil")
			return
		}
		if len(*metrics) != 0 {
			t.Fatalf("expected empty metrics, got (%#v)", metrics)
		}
		viper.Reset()
	}
}

func TestValidateStatsdOptions(t *testing.T) {
	t.Log("Testing validateStatsdOptions")

	viper.Set(config.KeyStatsdDisabled, true)

	t.Log("StatsD disabled")
	{
		err := validateStatsdOptions()
		if err != nil {
			t.Fatalf("Expected NO error, got (%v)", err)
		}
	}

	viper.Set(config.KeyStatsdDisabled, false)

	t.Log("StatsD port (invalid, empty)")
	{
		viper.Set(config.KeyStatsdPort, "")

		expectedErr := fmt.Errorf("invalid StatsD port (empty)") //nolint:goerr113
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("StatsD port (invalid, not a number)")
	{
		viper.Set(config.KeyStatsdPort, "abc")

		expectedErr := fmt.Errorf("invalid StatsD port (abc)") //nolint:goerr113
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("StatsD port (invalid, out of range, low)")
	{
		viper.Set(config.KeyStatsdPort, "10")

		expectedErr := fmt.Errorf("invalid StatsD port 1024>10<65535") //nolint:goerr113
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("StatsD port (invalid, out of range, high)")
	{
		viper.Set(config.KeyStatsdPort, "70000")

		expectedErr := fmt.Errorf("invalid StatsD port 1024>70000<65535") //nolint:goerr113
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	viper.Set(config.KeyStatsdPort, "8125")

	t.Log("Host category (invalid, empty)")
	{
		viper.Set(config.KeyStatsdHostCategory, "")

		expectedErr := fmt.Errorf("invalid StatsD host category (empty)") //nolint:goerr113
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	viper.Set(config.KeyStatsdHostCategory, "statsd")

	t.Log("Group CID, OK - none")
	{
		viper.Set(config.KeyStatsdGroupCID, "")
		err := validateStatsdOptions()
		if err != nil {
			t.Fatalf("Expected NO error, got (%v)", err)
		}
	}

	t.Log("Group CID (cosi, no cfg)")
	{
		viper.Set(config.KeyStatsdGroupCID, "cosi")

		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if !strings.HasPrefix(err.Error(), "load cosi cid for group:") {
			t.Errorf("unexpected error (%s)", err)
		}
	}

	t.Log("Group CID (invalid, abc)")
	{
		viper.Set(config.KeyStatsdGroupCID, "abc")

		expectedErr := fmt.Errorf("invalid StatsD Group Check ID (abc)") //nolint:goerr113
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("Group CID, valid - 123")
	{
		viper.Set(config.KeyStatsdGroupCID, "123")
		expectedErr := fmt.Errorf("invalid StatsD host/group prefix (both empty)") //nolint:goerr113
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("Group CID, valid - /check_bundle/123")
	{
		viper.Set(config.KeyStatsdGroupCID, "/check_bundle/123")
		expectedErr := fmt.Errorf("invalid StatsD host/group prefix (both empty)") //nolint:goerr113
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	viper.Set(config.KeyStatsdGroupCID, "/check_bundle/123")
	viper.Set(config.KeyStatsdHostPrefix, "host.")

	t.Log("Group prefix, invalid (same as host)")
	{
		viper.Set(config.KeyStatsdGroupPrefix, "host.")

		expectedErr := fmt.Errorf("invalid StatsD host/group prefix (same)") //nolint:goerr113
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	viper.Set(config.KeyStatsdGroupPrefix, "group.")

	t.Log("Group counter operator, invalid (empty)")
	{
		viper.Set(config.KeyStatsdGroupCounters, "")

		expectedErr := fmt.Errorf("invalid StatsD counter operator (empty)") //nolint:goerr113
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("Group counter operator, invalid ('multiply')")
	{
		viper.Set(config.KeyStatsdGroupCounters, "multiply")

		expectedErr := fmt.Errorf("invalid StatsD counter operator (multiply)") //nolint:goerr113
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("Group counter operator, invalid ('sum')")
	{
		viper.Set(config.KeyStatsdGroupCounters, "sum")

		expectedErr := fmt.Errorf("invalid StatsD gauge operator (empty)") //nolint:goerr113
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("Group counter operator, invalid ('average')")
	{
		viper.Set(config.KeyStatsdGroupCounters, "average")

		expectedErr := fmt.Errorf("invalid StatsD gauge operator (empty)") //nolint:goerr113
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("Group gauge operator, invalid (empty)")
	{
		viper.Set(config.KeyStatsdGroupGauges, "")

		expectedErr := fmt.Errorf("invalid StatsD gauge operator (empty)") //nolint:goerr113
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("Group gauge operator, invalid ('multiply')")
	{
		viper.Set(config.KeyStatsdGroupGauges, "multiply")

		expectedErr := fmt.Errorf("invalid StatsD gauge operator (multiply)") //nolint:goerr113
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("Group gauge operator, invalid ('sum')")
	{
		viper.Set(config.KeyStatsdGroupGauges, "sum")

		expectedErr := fmt.Errorf("invalid StatsD set operator (empty)") //nolint:goerr113
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("Group gauge operator, invalid ('average')")
	{
		viper.Set(config.KeyStatsdGroupGauges, "average")

		expectedErr := fmt.Errorf("invalid StatsD set operator (empty)") //nolint:goerr113
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("Group set operator, invalid (empty)")
	{
		viper.Set(config.KeyStatsdGroupSets, "")

		expectedErr := fmt.Errorf("invalid StatsD set operator (empty)") //nolint:goerr113
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("Group set operator, invalid ('multiply')")
	{
		viper.Set(config.KeyStatsdGroupSets, "multiply")

		expectedErr := fmt.Errorf("invalid StatsD set operator (multiply)") //nolint:goerr113
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("Group gauge operator, invalid ('sum')")
	{
		viper.Set(config.KeyStatsdGroupSets, "sum")

		err := validateStatsdOptions()
		if err != nil {
			t.Fatalf("Expected NO error, got (%v)", err)
		}
	}

	t.Log("Group gauge operator, invalid ('average')")
	{
		viper.Set(config.KeyStatsdGroupSets, "average")

		err := validateStatsdOptions()
		if err != nil {
			t.Fatalf("Expected NO error, got (%v)", err)
		}
	}

	viper.Reset()
}
