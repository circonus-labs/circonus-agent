// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetrics(t *testing.T) {
	t.Log("Testing Metrics")

	tests := []struct {
		name        string
		pluginID    string
		response    string
		shouldErr   bool
		expectedErr string
	}{
		{"invalid (plugin id)", "[invalid]", "", true, "invalid plugin id ([invalid])"},
		{"invalid (json/parse)", "", "invalid", true, "parsing metrics: invalid character 'i' looking for beginning of value"},
		{"valid", "", `{"foo":{"_type":"n", "_value":3.12}}`, false, ""},
		{"valid (plugin id)", "bar", "{\"bar`test\":{\"_type\":\"i\", \"_value\":1}}", false, ""},
	}

	for _, test := range tests {
		t.Log("\t", test.name)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(test.response))
		}))

		var c *Client
		var err error

		c, err = New(ts.URL)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}

		_, err = c.Metrics(test.pluginID)

		if test.shouldErr {
			if err == nil {
				t.Fatal("expected error")
			}
			if err.Error() != test.expectedErr {
				t.Fatalf("unexpected error (%s)", err)
			}
		} else {
			if err != nil {
				t.Fatalf("expected no error, got (%s)", err)
			}
		}

		ts.Close()
	}
}
