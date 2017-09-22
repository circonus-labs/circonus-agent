// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/plugins"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestRouter(t *testing.T) {
	t.Log("Testing router")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	{
		methods := []string{
			"CONNECT",
			"DELETE",
			"HEAD",
			"OPTIONS",
			"TRACE",
		}
		for _, method := range methods {
			t.Logf("Method not allowed (%s)", method)
			req := httptest.NewRequest(method, "/", nil)
			w := httptest.NewRecorder()

			s := New(nil, nil)
			s.router(w, req)

			resp := w.Result()

			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Fatalf("expected %d, got %d", http.StatusMethodNotAllowed, resp.StatusCode)
			}
		}
	}

	{
		viper.Set(config.KeyPluginDir, "testdata/")
		p := plugins.New()
		reqtests := []struct {
			method string
			path   string
		}{
			{"GET", "/invalid"},
			{"GET", "/run/invalid"},
			{"GET", "/inventory/invalid"},
			{"POST", "/invalid"},
			{"PUT", "/invalid"},
			{"PUT", "/write/"}, // /write/ must be followed by an id/name to use as "plugin namespace"
		}
		s := New(p, nil)
		for _, reqtest := range reqtests {
			t.Logf("Invalid path (%s %s)", reqtest.method, reqtest.path)
			req := httptest.NewRequest(reqtest.method, reqtest.path, nil)
			w := httptest.NewRecorder()

			s.router(w, req)

			resp := w.Result()

			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("expected %d, got %d", http.StatusNotFound, resp.StatusCode)
			}
		}
	}

	{
		viper.Set(config.KeyStatsdDisabled, true)
		viper.Set(config.KeyPluginDir, "testdata/")
		p := plugins.New()
		s := New(p, nil)
		reqtests := []struct {
			method string
			path   string
		}{
			{"GET", "/"},
			{"GET", "/run"},
			{"GET", "/run/"},
			{"GET", "/inventory"},
			{"GET", "/inventory/"},
		}
		for _, reqtest := range reqtests {
			t.Logf("OK path (%s %s)", reqtest.method, reqtest.path)
			req := httptest.NewRequest(reqtest.method, reqtest.path, nil)
			w := httptest.NewRecorder()

			s.router(w, req)

			resp := w.Result()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected %d, got %d", http.StatusOK, resp.StatusCode)
			}
		}
	}

	t.Log("Invalid (PUT /write/foo) w/o data")
	{
		viper.Set(config.KeyStatsdDisabled, true)
		viper.Set(config.KeyPluginDir, "testdata/")
		p := plugins.New()
		s := New(p, nil)

		req := httptest.NewRequest("PUT", "/write/foo", nil)
		w := httptest.NewRecorder()

		s.router(w, req)

		resp := w.Result()

		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected %d, got %d", http.StatusInternalServerError, resp.StatusCode)
		}
	}

	t.Log("OK (PUT /write/foo) w/data")
	{

		s := New(nil, nil)

		reqBody := bytes.NewReader([]byte(`{"test":1}`))

		req := httptest.NewRequest("PUT", "/write/foo", reqBody)
		w := httptest.NewRecorder()

		s.router(w, req)

		resp := w.Result()

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected %d, got %d", http.StatusNoContent, resp.StatusCode)
		}
	}
}
