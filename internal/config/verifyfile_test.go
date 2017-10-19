// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyFile(t *testing.T) {

	t.Log("empty")
	{
		_, err := verifyFile("")
		if err == nil {
			t.Fatal("expected error")
		}
		expect := "Invalid file name (empty)"
		if err.Error() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, err)
		}
	}

	t.Log("missing")
	{
		_, err := verifyFile(filepath.Join("testdata", "missing"))
		if err == nil {
			t.Fatal("expected error")
		}
		expect := "missing: no such file or directory"
		if !strings.Contains(err.Error(), expect) {
			t.Fatalf("expect (%s$) got (%s)", expect, err)
		}
	}

	t.Log("permissions")
	{
		_, err := verifyFile(filepath.Join("testdata", "no_access_dir", "test"))
		if err == nil {
			t.Fatal("expected error (verify mkdir -p testdata/no_access_dir/test && chmod -R 700 testdata/no_access_dir && chmown -R root testdata/no_access_dir)")
		}
		expect := "test: permission denied"
		if !strings.Contains(err.Error(), expect) {
			t.Fatalf("expect (%s$) got (%s)", expect, err)
		}
	}

	t.Log("not regular file")
	{
		_, err := verifyFile(filepath.Join("testdata", "no_access_dir"))
		if err == nil {
			t.Fatal("expected error (verify mkdir -p testdata/no_access_dir/test && chmod -R 700 testdata/no_access_dir && chmown -R root testdata/no_access_dir)")
		}
		expect := "no_access_dir: not a regular file"
		if !strings.Contains(err.Error(), expect) {
			t.Fatalf("expect (%s$) got (%s)", expect, err)
		}
	}

	t.Log("no access file")
	{
		_, err := verifyFile(filepath.Join("testdata", "no_access_file"))
		if err == nil {
			t.Fatal("expected error (verify touch testdata/no_access_file && chmod 600 testdata/no_access_file && chmown -R root testdata/no_access_file)")
		}
		expect := "no_access_file: permission denied"
		if !strings.Contains(err.Error(), expect) {
			t.Fatalf("expect (%s$) got (%s)", expect, err)
		}
	}

	t.Log("valid")
	{
		_, err := verifyFile(filepath.Join("testdata", "test.file"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}
}
