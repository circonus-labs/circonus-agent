// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestValidateLoadCOSICheckID(t *testing.T) {
	t.Log("Testing loadCOSICheckID")

	t.Log("No cosi config")
	{
		expectedErr := errors.New("Unable to access cosi check config: open testdata/cosi_check_missing.json: no such file or directory")
		cfgFile := filepath.Join("testdata", "cosi_check_missing.json")
		cid, err := loadCOSICheckID(cfgFile)
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
		expectedErr := errors.New("Unable to parse cosi check cosi config (testdata/cosi_bad.json): invalid character '#' looking for beginning of value")
		cfgFile := filepath.Join("testdata", "cosi_bad.json")
		cid, err := loadCOSICheckID(cfgFile)
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
		expectedErr := errors.New("Missing CID key, invalid cosi check config (testdata/cosi_check_invalid_cid.json)")
		cfgFile := filepath.Join("testdata", "cosi_check_invalid_cid.json")
		cid, err := loadCOSICheckID(cfgFile)
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
		cid, err := loadCOSICheckID(cfgFile)
		if err != nil {
			t.Fatalf("Expected NO error, got (%s)", err)
		}
		expectedCID := "/check_bundle/123"
		if cid != expectedCID {
			t.Errorf("expected '%s' got '%s'", expectedCID, cid)
		}
	}
}
