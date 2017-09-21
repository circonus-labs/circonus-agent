// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestValidatePluginDirectory(t *testing.T) {
	t.Log("Testing validatePluginDirectory")

	t.Log("No directory")
	{
		viper.Set(KeyPluginDir, "")
		expectedError := errors.New("Invalid plugin directory ()")
		err := validatePluginDirectory()
		if err == nil {
			t.Fatalf("Expected error")
		}
		if err.Error() != expectedError.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedError, err)
		}
	}

	t.Log("Invalid directory (not found)")
	{
		viper.Set(KeyPluginDir, "foo")
		err := validatePluginDirectory()
		if err == nil {
			t.Fatalf("Expected error")
		}
		sfx := "internal/config/foo: no such file or directory"
		if !strings.HasSuffix(err.Error(), sfx) {
			t.Errorf("Expected (%s) got (%s)", sfx, err)
		}
	}

	t.Log("Invalid directory (not a dir)")
	{
		viper.Set(KeyPluginDir, filepath.Join("testdata", "not_a_dir"))
		err := validatePluginDirectory()
		if err == nil {
			t.Fatalf("Expected error")
		}
		sfx := "internal/config/testdata/not_a_dir) not a directory"
		if !strings.HasSuffix(err.Error(), sfx) {
			t.Errorf("Expected (%s) got (%s)", sfx, err)
		}
	}

	//
	// NOTE next two will fail if the directory structure isn't set up correctly (which is not 'git'able)
	//
	// sudo mkdir -p testdata/no_access_dir/test && sudo chmod -R 700 testdata/no_access_dir
	//
	t.Log("Invalid directory (perms, subdir)")
	{
		viper.Set(KeyPluginDir, filepath.Join("testdata", "no_access_dir", "test"))
		err := validatePluginDirectory()
		if err == nil {
			t.Fatalf("Expected error - check 'sudo mkdir -p testdata/no_access_dir/test && sudo chmod -R 700 testdata/no_access_dir'")
		}
		sfx := "internal/config/testdata/no_access_dir/test: permission denied"
		if !strings.HasSuffix(err.Error(), sfx) {
			t.Errorf("Expected (%s) got (%s)", sfx, err)
		}
	}

	t.Log("Invalid directory (perms, open)")
	{
		viper.Set(KeyPluginDir, filepath.Join("testdata", "no_access_dir"))
		err := validatePluginDirectory()
		if err == nil {
			t.Fatalf("Expected error")
		}
		sfx := "internal/config/testdata/no_access_dir: permission denied"
		if !strings.HasSuffix(err.Error(), sfx) {
			t.Errorf("Expected (%s) got (%s)", sfx, err)
		}
	}

	t.Log("Valid directory")
	{
		viper.Set(KeyPluginDir, filepath.Join("testdata"))
		err := validatePluginDirectory()
		if err != nil {
			t.Fatal("Expected NO error")
		}
		dir := viper.GetString(KeyPluginDir)
		sfx := "internal/config/testdata"
		if !strings.HasSuffix(dir, sfx) {
			t.Errorf("Expected (%s), got '%s'", sfx, dir)
		}
	}
}
