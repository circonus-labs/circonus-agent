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

func TestInventory(t *testing.T) {
	t.Log("Testing Inventory")

	tests := []struct {
		name        string
		response    string
		shouldErr   bool
		expectedErr string
	}{
		{"invalid (json/parse)", "invalid", true, "parsing inventory: invalid character 'i' looking for beginning of value"},
		{"valid", `[{"id":"test","name":"test","instance":""}]`, false, ""},
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

		_, err = c.Inventory()

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
