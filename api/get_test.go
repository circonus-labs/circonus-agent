// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package api

import (
	"context"
	"errors"
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

	tests := []struct { //nolint:govet
		name           string
		rpath          string
		expectedErr    error
		expectedErrStr string
		shouldErr      bool
	}{
		{"invalid path (empty)", "", errInvalidRequestPath, "", true},
		{"invalid path (bad)", "/%/%", nil, `creating request URL: parse "/%/%": invalid URL escape "%/%"`, true},
		{"invalid path (not found)", "/not_found", errInvalidHTTPResponse, "", true}, // "404 Not Found - " + ts.URL + "/not_found - Not Found", true},
		{"valid", "/valid", nil, "", false},
	}

	for _, test := range tests {
		t.Log("\t", test.name)

		_, err := c.get(context.Background(), test.rpath)

		if test.shouldErr {
			switch {
			case err == nil:
				t.Fatal("expected error")
			case test.expectedErr != nil:
				if !errors.Is(err, test.expectedErr) {
					// if err.Error() != test.expectedErr {
					t.Fatalf("unexpected error (%s)", err)
				}
			case test.expectedErrStr != "":
				if err.Error() != test.expectedErrStr {
					t.Fatalf("unexpected error (%s)", err)
				}
			}
		} else if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
	}
}
