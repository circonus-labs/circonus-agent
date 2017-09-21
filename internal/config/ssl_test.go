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

func TestValidateSSLOptions(t *testing.T) {
	t.Log("Testing validateSSLOptions")

	crtFile := filepath.Join("testdata", "ssl_test.pem")
	keyFile := filepath.Join("testdata", "ssl_test.key")

	t.Log("Invalid ssl server listen spec")
	{
		viper.Set(KeySSLListen, "1.2.3")
		expectedErr := errors.New("Invalid IP address format specified '1.2.3'")
		err := validateSSLOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	viper.Set(KeySSLListen, "127.0.0.1:2610")

	t.Log("Invalid (dir as file)")
	{
		viper.Set(KeySSLCertFile, "testdata")

		expectedErr := errors.New("SSL cert: not a regular file")
		err := validateSSLOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected ends with (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("No cert")
	{
		viper.Set(KeySSLCertFile, "")

		expectedErr := errors.New("SSL cert: Invalid file name (empty)")
		err := validateSSLOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("Invalid cert")
	{
		viper.Set(KeySSLCertFile, crtFile+".missing")

		err := validateSSLOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		sfx := "internal/config/testdata/ssl_test.pem.missing: no such file or directory"
		if !strings.HasSuffix(err.Error(), sfx) {
			t.Errorf("Expected ends with (%s$) got (%s)", sfx, err)
		}
	}

	t.Log("No key")
	{
		viper.Set(KeySSLCertFile, crtFile)
		viper.Set(KeySSLKeyFile, "")

		expectedErr := errors.New("SSL key: Invalid file name (empty)")
		err := validateSSLOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("Invalid key")
	{
		viper.Set(KeySSLCertFile, crtFile)
		viper.Set(KeySSLKeyFile, keyFile+".missing")

		err := validateSSLOptions()
		if err == nil {
			t.Fatal("Expected error")
		}
		sfx := "internal/config/testdata/ssl_test.key.missing: no such file or directory"
		if !strings.HasSuffix(err.Error(), sfx) {
			t.Errorf("Expected ends with (%s$) got (%s)", sfx, err)
		}
	}

	t.Log("Valid")
	{
		viper.Set(KeySSLCertFile, crtFile)
		viper.Set(KeySSLKeyFile, keyFile)

		err := validateSSLOptions()
		if err != nil {
			t.Fatalf("Expected NO error, got (%s)", err)
		}

		crt := viper.GetString(KeySSLCertFile)
		key := viper.GetString(KeySSLKeyFile)

		if !strings.HasSuffix(crt, crtFile) {
			t.Errorf("Expected end with '%s$' crt, got '%s'", crtFile, crt)
		}
		if !strings.HasSuffix(key, keyFile) {
			t.Errorf("Expected end with '%s$' key, got '%s'", keyFile, key)
		}
	}
}
