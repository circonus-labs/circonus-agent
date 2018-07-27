// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"path/filepath"
	"testing"

	"github.com/pkg/errors"
)

func TestIsValidCheckID(t *testing.T) {
	t.Log("Testing IsValidCheckID")

	t.Log("valid - short")
	{
		ok, err := IsValidCheckID("1234")
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
		if !ok {
			t.Fatal("expected ok=true")
		}
	}

	t.Log("valid - long")
	{
		ok, err := IsValidCheckID("/check_bundle/1234")
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
		if !ok {
			t.Fatal("expected ok=true")
		}
	}

	t.Log("invalid")
	{
		ok, err := IsValidCheckID("foo")
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
		if ok {
			t.Fatal("expected ok=false")
		}
	}
}

func TestLoadCosiV1Config(t *testing.T) {
	t.Log("Testing loadCosiV1Config")

	tests := []struct {
		name        string
		file        string
		shouldFail  bool
		expectedErr string
	}{
		{"invalid (missing)", filepath.Join("testdata", "cosi_missing.json"), true, "unable to access cosi config: open testdata/cosi_missing.json: no such file or directory"},
		{"invalid (bad)", filepath.Join("testdata", "cosi_bad.json"), true, "unable to parse cosi config (testdata/cosi_bad.json): invalid character '#' looking for beginning of value"},
		{"invalid (missing key)", filepath.Join("testdata", "cosiv1_invalid_key.json"), true, "missing API key, invalid cosi config (testdata/cosiv1_invalid_key.json)"},
		{"invalid (missing app)", filepath.Join("testdata", "cosiv1_invalid_app.json"), true, "missing API app, invalid cosi config (testdata/cosiv1_invalid_app.json)"},
		{"invalid (missing url)", filepath.Join("testdata", "cosiv1_invalid_url.json"), true, "missing API URL, invalid cosi config (testdata/cosiv1_invalid_url.json)"},
		{"valid", filepath.Join("testdata", "cosiv1.json"), false, ""},
	}

	for _, test := range tests {
		tst := test
		t.Run(tst.name, func(t *testing.T) {
			t.Parallel()
			_, err := loadCosiV1Config(tst.file)
			if tst.shouldFail {
				if err == nil {
					t.Fatal("expected error")
				} else if err.Error() != tst.expectedErr {
					t.Fatalf("unexpected error (%s)", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error (%s)", err)
				}
			}
		})
	}
}

func TestLoadCosiV2Config(t *testing.T) {
	t.Log("Testing loadCosiV1Config")

	tests := []struct {
		name        string
		file        string
		shouldFail  bool
		expectedErr string
	}{
		{"invalid (missing)", filepath.Join("testdata", "cosi_missing"), true, "unable to load cosi config: no config found matching (testdata/cosi_missing.json|.toml|.yaml)"},
		{"invalid (bad)", filepath.Join("testdata", "cosi_bad"), true, "unable to load cosi config: parsing configuration file (testdata/cosi_bad.json): invalid character '#' looking for beginning of value"},
		{"invalid (missing key)", filepath.Join("testdata", "cosiv2_invalid_key"), true, "missing API key, invalid cosi config (testdata/cosiv2_invalid_key)"},
		{"invalid (missing app)", filepath.Join("testdata", "cosiv2_invalid_app"), true, "missing API app, invalid cosi config (testdata/cosiv2_invalid_app)"},
		{"invalid (missing url)", filepath.Join("testdata", "cosiv2_invalid_url"), true, "missing API URL, invalid cosi config (testdata/cosiv2_invalid_url)"},
		{"valid", filepath.Join("testdata", "cosiv2"), false, ""},
	}

	for _, test := range tests {
		tst := test
		t.Run(tst.name, func(t *testing.T) {
			t.Parallel()
			_, err := loadCosiV2Config(tst.file)
			if tst.shouldFail {
				if err == nil {
					t.Fatal("expected error")
				} else if err.Error() != tst.expectedErr {
					t.Fatalf("unexpected error (%s)", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error (%s)", err)
				}
			}
		})
	}
}

func TestLoadCheckConfig(t *testing.T) {
	t.Log("Testing loadCheckConfig")

	t.Log("No cosi config")
	{
		cfgFile := filepath.Join("testdata", "cosi_check_missing.json")
		expectedErr := errors.Errorf("unable to access cosi check config: open %s: no such file or directory", cfgFile)
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
		expectedErr := errors.Errorf("unable to parse cosi check cosi config (%s): invalid character '#' looking for beginning of value", cfgFile)
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
		expectedErr := errors.Errorf("missing CID key, invalid cosi check config (%s)", cfgFile)
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
