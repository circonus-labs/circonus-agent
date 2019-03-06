// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package tags

import (
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/spf13/viper"
)

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

func TestGetBaseTags(t *testing.T) {
	t.Log("Testing GetBaseTags")

	viper.Set(config.KeyCheckMetricStreamtags, true)

	// straight tags (basic operation)
	viper.Set(config.KeyCheckTags, "c1:v1,c2:v1")
	tags := GetBaseTags()
	if len(tags) != 2 {
		t.Fatalf("expected two tags, got (%d)", len(tags))
	}
	if tags[0] != "c1:v1" {
		t.Fatalf("expected c1:v1, got (%s)", tags[0])
	}
	if tags[1] != "c2:v1" {
		t.Fatalf("expected c2:v1, got (%s)", tags[1])
	}

	// systemd ExecStart quote oddity
	viper.Set(config.KeyCheckTags, `"c1:v1,c2:v2"`)
	tags = GetBaseTags()
	if len(tags) != 2 {
		t.Fatalf("expected two tags, got (%d)", len(tags))
	}
	if tags[0] != "c1:v1" {
		t.Fatalf("expected c1:v1, got (%s)", tags[0])
	}
	if tags[1] != "c2:v1" {
		t.Fatalf("expected c2:v1, got (%s)", tags[1])
	}
}
