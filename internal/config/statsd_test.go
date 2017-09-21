// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func TestValidateStatsdOptions(t *testing.T) {
	t.Log("Testing validateStatsdOptions")

	viper.Set(KeyStatsdDisabled, true)

	t.Log("StatsD disabled")
	{
		err := validateStatsdOptions()
		if err != nil {
			t.Fatalf("Expected NO error, got (%v)", err)
		}
	}

	viper.Set(KeyStatsdDisabled, false)

	t.Log("StatsD port (invalid, empty)")
	{
		viper.Set(KeyStatsdPort, "")

		expectedErr := errors.New("Invalid StatsD port (empty)")
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
		viper.Set(KeyStatsdPort, "abc")

		expectedErr := errors.New("Invalid StatsD port (abc)")
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
		viper.Set(KeyStatsdPort, "10")

		expectedErr := errors.New("Invalid StatsD port 1024>10<65535")
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
		viper.Set(KeyStatsdPort, "70000")

		expectedErr := errors.New("Invalid StatsD port 1024>70000<65535")
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	viper.Set(KeyStatsdPort, "8125")

	t.Log("Host category (invalid, empty)")
	{
		viper.Set(KeyStatsdHostCategory, "")

		expectedErr := errors.New("Invalid StatsD host category (empty)")
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	viper.Set(KeyStatsdHostCategory, "statsd")

	t.Log("Group CID, OK - none")
	{
		viper.Set(KeyStatsdGroupCID, "")
		err := validateStatsdOptions()
		if err != nil {
			t.Fatalf("Expected NO error, got (%v)", err)
		}
	}

	t.Log("Group CID (cosi, no cfg)")
	{
		viper.Set(KeyStatsdGroupCID, "cosi")

		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if !strings.HasPrefix(err.Error(), "Unable to access cosi check config:") {
			t.Errorf("Expected (%s) got (%s)", "Unable to access cosi check config: ...", err)
		}
	}

	t.Log("Group CID (invalid, abc)")
	{
		viper.Set(KeyStatsdGroupCID, "abc")

		expectedErr := errors.New("StatsD Group Check ID: Invalid Check ID (abc)")
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
		viper.Set(KeyStatsdGroupCID, "123")
		expectedErr := errors.New("StatsD host/group prefix mismatch (both empty)")
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
		viper.Set(KeyStatsdGroupCID, "/check_bundle/123")
		expectedErr := errors.New("StatsD host/group prefix mismatch (both empty)")
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	viper.Set(KeyStatsdGroupCID, "/check_bundle/123")
	viper.Set(KeyStatsdHostPrefix, "host.")

	t.Log("Group prefix, invalid (same as host)")
	{
		viper.Set(KeyStatsdGroupPrefix, "host.")

		expectedErr := errors.New("StatsD host/group prefix mismatch (same)")
		err := validateStatsdOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	viper.Set(KeyStatsdGroupPrefix, "group.")

	t.Log("Group counter operator, invalid (empty)")
	{
		viper.Set(KeyStatsdGroupCounters, "")

		expectedErr := errors.New("Invalid StatsD counter operator (empty)")
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
		viper.Set(KeyStatsdGroupCounters, "multiply")

		expectedErr := errors.New("Invalid StatsD counter operator (multiply)")
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
		viper.Set(KeyStatsdGroupCounters, "sum")

		expectedErr := errors.New("Invalid StatsD gauge operator (empty)")
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
		viper.Set(KeyStatsdGroupCounters, "average")

		expectedErr := errors.New("Invalid StatsD gauge operator (empty)")
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
		viper.Set(KeyStatsdGroupGauges, "")

		expectedErr := errors.New("Invalid StatsD gauge operator (empty)")
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
		viper.Set(KeyStatsdGroupGauges, "multiply")

		expectedErr := errors.New("Invalid StatsD gauge operator (multiply)")
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
		viper.Set(KeyStatsdGroupGauges, "sum")

		expectedErr := errors.New("Invalid StatsD set operator (empty)")
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
		viper.Set(KeyStatsdGroupGauges, "average")

		expectedErr := errors.New("Invalid StatsD set operator (empty)")
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
		viper.Set(KeyStatsdGroupSets, "")

		expectedErr := errors.New("Invalid StatsD set operator (empty)")
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
		viper.Set(KeyStatsdGroupSets, "multiply")

		expectedErr := errors.New("Invalid StatsD set operator (multiply)")
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
		viper.Set(KeyStatsdGroupSets, "sum")

		err := validateStatsdOptions()
		if err != nil {
			t.Fatalf("Expected NO error, got (%v)", err)
		}
	}

	t.Log("Group gauge operator, invalid ('average')")
	{
		viper.Set(KeyStatsdGroupSets, "average")

		err := validateStatsdOptions()
		if err != nil {
			t.Fatalf("Expected NO error, got (%v)", err)
		}
	}

}
