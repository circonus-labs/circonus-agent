// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestValidateReverseOptions(t *testing.T) {
	t.Log("Testing validateReverseOptions")

	t.Log("Reverse, (OK, no cid)")
	{
		err := validateReverseOptions()
		if err != nil {
			t.Fatalf("Expected NO error, got (%v)", err)
		}
	}

	t.Log("Reverse, (invalid, abc)")
	{
		expectedErr := errors.New("Invalid Reverse Check ID (abc)")
		viper.Set(KeyCheckBundleID, "abc")
		err := validateReverseOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("Reverse, (invalid, /check_bundle/abc)")
	{
		expectedErr := errors.New("Invalid Reverse Check ID (/check_bundle/abc)")
		viper.Set(KeyCheckBundleID, "/check_bundle/abc")
		err := validateReverseOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("Reverse, (valid, short, 123)")
	{
		viper.Set(KeyCheckBundleID, "123")
		err := validateReverseOptions()
		if err != nil {
			t.Fatalf("Expected NO error, got (%v)", err)
		}
	}

	t.Log("Reverse, (valid, long, /check_bundle/123)")
	{
		viper.Set(KeyCheckBundleID, "/check_bundle/123")
		err := validateReverseOptions()
		if err != nil {
			t.Fatalf("Expected NO error, got (%v)", err)
		}
	}

	t.Log("Reverse, ('cosi')")
	{
		viper.Set(KeyCheckBundleID, "cosi")
		err := validateReverseOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if !strings.HasPrefix(err.Error(), "unable to access cosi check config:") {
			t.Errorf("unexpected error (%s)", err)
		}
	}
}
