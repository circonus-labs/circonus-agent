// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"io/ioutil"
	"testing"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestValidate(t *testing.T) {
	t.Log("Testing validate")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("no config")
	{
		err := Validate()
		if err != nil {
			t.Fatalf("Expected NO error, got (%s)", err)
		}
	}

	t.Log("reverse")
	{
		viper.Set(KeyReverse, true)
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
