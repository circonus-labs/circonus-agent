// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
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
		name           string
		file           string
		expectedErr    error
		expectedErrStr string
		shouldFail     bool
	}{
		{"invalid (missing)", filepath.Join("testdata", "cosi_missing.json"), nil, "unable to access cosi config: open testdata/cosi_missing.json: no such file or directory", true},
		{"invalid (bad)", filepath.Join("testdata", "cosi_bad.json"), nil, "json parse - cosi config (testdata/cosi_bad.json): invalid character '#' looking for beginning of value", true},
		{"invalid (missing key)", filepath.Join("testdata", "cosiv1_invalid_key.json"), errMissingAPIKey, "", true},
		{"invalid (missing app)", filepath.Join("testdata", "cosiv1_invalid_app.json"), errMissingAPIApp, "", true},
		{"invalid (missing url)", filepath.Join("testdata", "cosiv1_invalid_url.json"), errMissingAPIURL, "", true},
		{"valid", filepath.Join("testdata", "cosiv1.json"), nil, "", false},
	}

	for _, test := range tests {
		tst := test
		t.Run(tst.name, func(t *testing.T) {
			t.Parallel()
			_, err := loadCosiV1Config(tst.file)
			if tst.shouldFail {
				switch {
				case err == nil:
					t.Fatal("expected error")
				case tst.expectedErr != nil:
					if !errors.Is(err, tst.expectedErr) {
						t.Fatalf("unexpected error (%s)", err)
					}
				case tst.expectedErrStr != "":
					if err.Error() != tst.expectedErrStr {
						t.Fatalf("unexpected error (%s)", err)
					}
				}
			} else if err != nil {
				t.Fatalf("unexpected error (%s)", err)
			}
		})
	}
}

func TestLoadCosiV2Config(t *testing.T) {
	t.Log("Testing loadCosiV1Config")

	tests := []struct {
		name           string
		file           string
		expectedErr    error
		expectedErrStr string
		shouldFail     bool
	}{
		{"invalid (missing)", filepath.Join("testdata", "cosi_missing"), nil, "unable to load cosi config: no config found matching (testdata/cosi_missing.json|.toml|.yaml): file does not exist", true},
		{"invalid (bad)", filepath.Join("testdata", "cosi_bad"), nil, "unable to load cosi config: parsing configuration file (testdata/cosi_bad.json): invalid character '#' looking for beginning of value", true},
		{"invalid (missing key)", filepath.Join("testdata", "cosiv2_invalid_key"), errMissingAPIKey, "", true},
		{"invalid (missing app)", filepath.Join("testdata", "cosiv2_invalid_app"), errMissingAPIApp, "", true},
		{"invalid (missing url)", filepath.Join("testdata", "cosiv2_invalid_url"), errMissingAPIURL, "", true},
		{"valid", filepath.Join("testdata", "cosiv2"), nil, "", false},
	}

	for _, test := range tests {
		tst := test
		t.Run(tst.name, func(t *testing.T) {
			t.Parallel()
			_, err := loadCosiV2Config(tst.file)
			if tst.shouldFail {
				switch {
				case err == nil:
					t.Fatal("expected error")
				case tst.expectedErr != nil:
					if !errors.Is(err, tst.expectedErr) {
						t.Fatalf("unexpected error (%s)", err)
					}
				case tst.expectedErrStr != "":
					if err.Error() != tst.expectedErrStr {
						t.Fatalf("unexpected error (%s)", err)
					}
				}
			} else if err != nil {
				t.Fatalf("unexpected error (%s)", err)
			}
		})
	}
}

func TestLoadCheckConfig(t *testing.T) {
	t.Log("Testing loadCheckConfig")

	t.Log("No cosi config")
	{
		cfgFile := filepath.Join("testdata", "cosi_check_missing.json")
		expectedErr := fmt.Errorf("unable to access cosi check config: open %s: no such file or directory", cfgFile) //nolint:goerr113
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
		expectedErr := fmt.Errorf("json parse - cosi check cosi config (%s): invalid character '#' looking for beginning of value", cfgFile) //nolint:goerr113
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
		expectedErr := fmt.Errorf("missing CID key, invalid cosi check config (%s)", cfgFile) //nolint:goerr113
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
