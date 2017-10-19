// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package cosi

import (
	"path/filepath"
	"testing"

	"github.com/pkg/errors"
)

func TestValidCheckID(t *testing.T) {
	t.Log("Testing ValidCheckID")

	t.Log("valid - short")
	{
		ok, err := ValidCheckID("1234")
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
		if !ok {
			t.Fatal("expected ok=true")
		}
	}

	t.Log("valid - long")
	{
		ok, err := ValidCheckID("/check_bundle/1234")
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
		if !ok {
			t.Fatal("expected ok=true")
		}
	}

	t.Log("invalid")
	{
		ok, err := ValidCheckID("foo")
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
		if ok {
			t.Fatal("expected ok=false")
		}
	}
}

func TestLoadCosiConfig(t *testing.T) {
	t.Log("Testing loadCosiConfig")

	t.Log("cosi - missing")
	{
		cfgFile := filepath.Join("testdata", "cosi_missing.json")
		expected := errors.Errorf("Unable to access cosi config: open %s: no such file or directory", cfgFile)
		_, err := loadCosiConfig(cfgFile)
		if err == nil {
			t.Fatalf("Expected error")
		}
		if err.Error() != expected.Error() {
			t.Errorf("Expected (%s) got (%s)", expected, err)
		}
	}

	t.Log("cosi - bad json")
	{
		cfgFile := filepath.Join("testdata", "cosi_bad.json")
		expected := errors.Errorf("Unable to parse cosi config (%s): invalid character '#' looking for beginning of value", cfgFile)
		_, err := loadCosiConfig(cfgFile)
		if err == nil {
			t.Fatalf("Expected error")
		}
		if err.Error() != expected.Error() {
			t.Errorf("Expected (%s) got (%s)", expected, err)
		}
	}

	t.Log("cosi - invalid config missing key")
	{
		cfgFile := filepath.Join("testdata", "cosi_invalid_key.json")
		expected := errors.Errorf("Missing API key, invalid cosi config (%s)", cfgFile)
		_, err := loadCosiConfig(cfgFile)
		if err == nil {
			t.Fatalf("Expected error")
		}
		if err.Error() != expected.Error() {
			t.Errorf("Expected (%s) got (%s)", expected, err)
		}
	}

	t.Log("cosi - invalid config missing app")
	{
		cfgFile := filepath.Join("testdata", "cosi_invalid_app.json")
		expected := errors.Errorf("Missing API app, invalid cosi config (%s)", cfgFile)
		_, err := loadCosiConfig(cfgFile)
		if err == nil {
			t.Fatalf("Expected error")
		}
		if err.Error() != expected.Error() {
			t.Errorf("Expected (%s) got (%s)", expected, err)
		}
	}

	t.Log("cosi - invalid config missing url")
	{
		cfgFile := filepath.Join("testdata", "cosi_invalid_url.json")
		expected := errors.Errorf("Missing API URL, invalid cosi config (%s)", cfgFile)
		_, err := loadCosiConfig(cfgFile)
		if err == nil {
			t.Fatalf("Expected error")
		}
		if err.Error() != expected.Error() {
			t.Errorf("Expected (%s) got (%s)", expected, err)
		}
	}

	t.Log("cosi - valid")
	{
		cfgFile := filepath.Join("testdata", "cosi.json")
		cfg, err := loadCosiConfig(cfgFile)
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}

		if cfg.Key == "" {
			t.Fatal("expected API Key")
		}
		if cfg.App == "" {
			t.Fatal("expected API App")
		}
		if cfg.URL == "" {
			t.Fatal("expected API URL")
		}
	}
}

func TestLoadCheckConfig(t *testing.T) {
	t.Log("Testing loadCheckConfig")

	t.Log("No cosi config")
	{
		cfgFile := filepath.Join("testdata", "cosi_check_missing.json")
		expectedErr := errors.Errorf("Unable to access cosi check config: open %s: no such file or directory", cfgFile)
		cid, err := loadCheckConfig(cfgFile)
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("expected '%s' got '%s'", expectedErr.Error(), err.Error())
		}
		if cid != "" {
			t.Errorf("expected blank got '%s'", cid)
		}
	}

	t.Log("Invalid cosi config (json)")
	{
		cfgFile := filepath.Join("testdata", "cosi_bad.json")
		expectedErr := errors.Errorf("Unable to parse cosi check cosi config (%s): invalid character '#' looking for beginning of value", cfgFile)
		cid, err := loadCheckConfig(cfgFile)
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("expected '%s' got '%s'", expectedErr.Error(), err.Error())
		}
		if cid != "" {
			t.Errorf("expected blank got '%s'", cid)
		}
	}

	t.Log("Invalid cosi config (missing cid)")
	{
		cfgFile := filepath.Join("testdata", "cosi_check_invalid_cid.json")
		expectedErr := errors.Errorf("Missing CID key, invalid cosi check config (%s)", cfgFile)
		cid, err := loadCheckConfig(cfgFile)
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("expected '%s' got '%s'", expectedErr.Error(), err.Error())
		}
		if cid != "" {
			t.Errorf("expected blank got '%s'", cid)
		}
	}

	t.Log("Valid cosi config")
	{
		cfgFile := filepath.Join("testdata", "cosi_check.json")
		cid, err := loadCheckConfig(cfgFile)
		if err != nil {
			t.Fatalf("Expected NO error, got (%s)", err)
		}
		expectedCID := "/check_bundle/123"
		if cid != expectedCID {
			t.Errorf("expected '%s' got '%s'", expectedCID, cid)
		}
	}
}
