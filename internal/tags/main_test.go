// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package tags

import "testing"

func TestPrepStreamTags(t *testing.T) {
	t.Log("Testing PrepStreamTags")

	tt := []struct {
		name        string
		tags        string
		expect      string
		shouldError bool
	}{
		{"valid - one tag", "c1:v1", "|ST[c1:v1]", false},
		{"valid - >1 tags", "c1:v1,c2:v2", "|ST[c1:v1,c2:v2]", false},
		{"valid - no tags", "", "", false},
		{"valid - char replace", "[\"foo\"]:`bar'baz'", "|ST[__foo__:_bar_baz_]", false},
		{"invalid - multi, no sep", "c1:v1c2:v2", "", true},
		{"invalid - no delim", "foo", "", true},
		{"invalid - bad delim", "foo|bar", "", true},
		{"invalid - multi delim", "foo:b:ar", "", true},
		{"invalid - multi sep", "foo:bar,c:v,", "", true},
		{"invalid - multi sep", "foo:bar,c,:v", "", true},
	}

	for _, tst := range tt {
		t.Logf("\ttest -- %s (%s)", tst.name, tst.tags)

		result, err := PrepStreamTags(tst.tags)
		if tst.shouldError {
			if err == nil {
				t.Fatal("expected error")
			}
		} else {
			if err != nil {
				t.Fatalf("expected no error, got (%s)", err)
			}
		}

		if result != tst.expect {
			t.Fatalf("expected (%s) got (%s)", tst.expect, result)
		}
	}
}
