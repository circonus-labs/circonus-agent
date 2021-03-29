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
		expectedErr string
		shouldErr   bool
	}{
		{"invalid (json/parse)", "invalid", "parsing inventory: invalid character 'i' looking for beginning of value", true},
		{"valid", `[{"id":"test","name":"test","instance":""}]`, "", false},
	}

	for _, test := range tests {
		resp := test.response
		t.Log("\t", test.name)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(resp))
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
		} else if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}

		ts.Close()
	}
}
