// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package api

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWrite(t *testing.T) {
	t.Log("Testing Write")

	tests := []struct {
		name        string
		id          string
		metrics     *Metrics
		shouldErr   bool
		expectedErr string
	}{
		{"invalid (id)", "", nil, true, "invalid group id (empty)"},
		{"invalid (nil metrics)", "foo", nil, true, "invalid metrics (nil)"},
		{"invalid (no metrics)", "foo", &Metrics{}, true, "invalid metrics (none)"},
		{"valid", "foo", &Metrics{"foo": Metric{"n", 3.12}}, false, ""},
	}

	for _, test := range tests {
		t.Log("\t", test.name)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()
			data, err := ioutil.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("expected no error, got (%s)", err)
			}
			_, _ = w.Write(data)
		}))

		var c *Client
		var err error

		c, err = New(ts.URL)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}

		err = c.Write(test.id, test.metrics)

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
