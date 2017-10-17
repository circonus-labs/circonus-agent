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

func TestValidateAPIOptions(t *testing.T) {
	t.Log("Testing validateAPIOptions")

	t.Log("API required (reverse)")
	{
		viper.Set(KeyReverse, true)
		yes := apiRequired()
		if !yes {
			t.Fatal("Expected true")
		}
		viper.Set(KeyReverse, false)
	}

	t.Log("API required (statsd w/group cid)")
	{
		viper.Set(KeyStatsdGroupCID, "123")
		yes := apiRequired()
		if !yes {
			t.Fatal("Expected true")
		}
	}

	t.Log("API required (reverse disabled, statsd disabled)")
	{
		viper.Set(KeyReverse, false)
		viper.Set(KeyStatsdDisabled, true)
		yes := apiRequired()
		if yes {
			t.Fatal("Expected false")
		}
		viper.Set(KeyStatsdDisabled, false)
	}

	t.Log("No key/app/url")
	{
		expectedError := errors.New("API key is required")
		err := validateAPIOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedError.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedError, err)
		}
	}

	t.Log("key=cosi, no cfg")
	{
		viper.Set(KeyAPITokenKey, cosiName)
		err := validateAPIOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		pfx := "Unable to access cosi config:"
		if !strings.HasPrefix(err.Error(), pfx) {
			t.Errorf("Expected (^%s) got (%s)", pfx, err)
		}
	}

	t.Log("No app")
	{
		viper.Set(KeyAPITokenKey, "foo")
		expectedError := errors.New("API app is required")
		err := validateAPIOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedError.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedError, err)
		}
	}

	t.Log("No url")
	{
		viper.Set(KeyAPITokenKey, "foo")
		viper.Set(KeyAPITokenApp, "foo")
		expectedError := errors.New("API URL is required")
		err := validateAPIOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedError.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedError, err)
		}
	}

	t.Log("Invalid url (foo)")
	{
		viper.Set(KeyAPITokenKey, "foo")
		viper.Set(KeyAPITokenApp, "foo")
		viper.Set(KeyAPIURL, "foo")
		expectedError := errors.New("Invalid API URL (foo)")
		err := validateAPIOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedError.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedError, err)
		}
	}

	t.Log("Invalid url (foo_bar://herp/derp)")
	{
		viper.Set(KeyAPITokenKey, "foo")
		viper.Set(KeyAPITokenApp, "foo")
		viper.Set(KeyAPIURL, "foo_bar://herp/derp")
		expectedError := errors.New("Invalid API URL: parse foo_bar://herp/derp: first path segment in URL cannot contain colon")
		err := validateAPIOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedError.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedError, err)
		}
	}

	t.Log("Valid options")
	{
		viper.Set(KeyAPITokenKey, "foo")
		viper.Set(KeyAPITokenApp, "foo")
		viper.Set(KeyAPIURL, "http://foo.com/bar")
		err := validateAPIOptions()
		if err != nil {
			t.Fatalf("Expected NO error, got (%s)", err)
		}
	}

}
