// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"errors"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestValidate(t *testing.T) {
	t.Log("Testing validate")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	viper.Set(KeyStatsdDisabled, true)

	t.Log("No plugin dir")
	{
		viper.Set(KeyPluginDir, "")
		expectedErr := errors.New("plugin directory config: Invalid plugin directory ()")
		err := Validate()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("Valid plugin directory")
	{
		viper.Set(KeyPluginDir, "testdata")
		err := Validate()
		if err != nil {
			t.Fatalf("Expected NO error, got (%s)", err)
		}
	}

	t.Log("Invalid server listen spec")
	{
		viper.Set(KeyListen, "1.2.3")
		expectedErr := errors.New("server config: Invalid IP address format specified '1.2.3'")
		err := Validate()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
		viper.Set("listen", "")
	}

	t.Log("Invalid ssl server listen spec")
	{
		viper.Set(KeySSLListen, "1.2.3")
		expectedErr := errors.New("ssl server config: Invalid IP address format specified '1.2.3'")
		err := Validate()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("Invalid ssl server config")
	{
		expectedErr := errors.New("ssl server config: SSL cert: Invalid file name (empty)")
		viper.Set(KeySSLListen, "127.0.0.1:2610")
		err := Validate()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("Valid ssl server config")
	{
		viper.Set(KeySSLListen, "127.0.0.1:2610")
		viper.Set(KeySSLCertFile, filepath.Join("testdata", "ssl_test.pem"))
		viper.Set(KeySSLKeyFile, filepath.Join("testdata", "ssl_test.key"))
		err := Validate()
		if err != nil {
			t.Fatalf("Expected NO error, got (%s)", err)
		}
	}
}

func TestShowConfig(t *testing.T) {
	t.Log("Testing ShowConfig")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("YAML")
	{
		viper.Set(KeyShowConfig, "yaml")
		err := ShowConfig(ioutil.Discard)
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
	}

	t.Log("TOML")
	{
		viper.Set(KeyShowConfig, "toml")
		err := ShowConfig(ioutil.Discard)
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
	}

	t.Log("JSON")
	{
		viper.Set(KeyShowConfig, "json")
		err := ShowConfig(ioutil.Discard)
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
	}
}

func TestGetConfig(t *testing.T) {
	t.Log("Testing getConfig")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	cfg, err := getConfig()
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}
	if cfg == nil {
		t.Fatal("expected not nil")
	}
}

func TestStatConfig(t *testing.T) {
	t.Log("Testing StatConfig")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	err := StatConfig()
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}
}
