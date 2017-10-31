// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import "testing"

type config struct {
	ID string `json:"id" toml:"id" yaml:"id"`
}

func TestLoadConfigFile(t *testing.T) {
	t.Log("Testing LoadConfigFile")

	tt := []struct {
		name        string
		base        string
		expectError bool
	}{
		{"JSON", "testdata/test_cfg_json", false},
		{"TOML", "testdata/test_cfg_toml", false},
		{"YAML", "testdata/test_cfg_yaml", false},
		{"empty", "", true},
		{"missing", "testdata/test_cfg_missing", true},
		{"JSON error", "testdata/test_cfg_json_error", true},
		{"TOML error", "testdata/test_cfg_toml_error", true},
		{"YAML error", "testdata/test_cfg_yaml_error", true},
	}

	for _, tst := range tt {
		t.Logf("\t%s", tst.name)
		var c config
		err := LoadConfigFile(tst.base, &c)
		if tst.expectError && err == nil {
			t.Fatalf("expected error for %s", tst.base)
		}
		if !tst.expectError && err != nil {
			t.Fatalf("expected no error, got (%s), loading (%s)", err, tst.base)
		}
	}
}
