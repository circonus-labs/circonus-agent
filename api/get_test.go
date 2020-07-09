// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGet(t *testing.T) {
	t.Log("Testing get")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch p {
		case "/valid":
			_, _ = w.Write([]byte(`valid`))
		default:
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c, err := New(ts.URL)
	if err != nil {
		t.Fatalf("unexpected error (%s)", err)
	}

	tests := []struct {
		name        string
		rpath       string
		shouldErr   bool
		expectedErr string
	}{
		{"invalid path (empty)", "", true, "invalid request path (empty)"},
		{"invalid path (bad)", "/%/%", true, "creating request url: parse /%/%: invalid URL escape \"%/%\""},
		{"invalid path (not found)", "/not_found", true, "404 Not Found - " + ts.URL + "/not_found - Not Found"},
		{"valid", "/valid", false, ""},
	}

	for _, test := range tests {
		t.Log("\t", test.name)

		_, err := c.get(context.Background(), test.rpath)

		if test.shouldErr {
			if err == nil {
				t.Fatal("expected error")
			}
			if err.Error() != test.expectedErr {
				t.Fatalf("unexpected error (%s)", err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
	}
}
