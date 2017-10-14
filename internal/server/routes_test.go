// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"bytes"
	"context"
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
	viper.Reset()
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("bad methods")
	{
		methods := []string{
			"CONNECT",
			"DELETE",
			"HEAD",
			"OPTIONS",
			"TRACE",
		}
		s, _ := New(nil, nil)
		for _, method := range methods {
			t.Logf("Method not allowed (%s)", method)
			req := httptest.NewRequest(method, "/", nil)
			w := httptest.NewRecorder()
			s.router(w, req)
			resp := w.Result()
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Fatalf("expected %d, got %d", http.StatusMethodNotAllowed, resp.StatusCode)
			}
		}
		viper.Reset()
	}

	t.Log("invalid paths")
	{
		viper.Reset()
		viper.Set(config.KeyPluginDir, "testdata/")
		p := plugins.New(context.Background())
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
		s, _ := New(p, nil)
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
		viper.Reset()
	}

	t.Log("valid")
	{
		viper.Set(config.KeyStatsdDisabled, true)
		viper.Set(config.KeyPluginDir, "testdata/")
		p := plugins.New(context.Background())
		s, _ := New(p, nil)
		reqtests := []struct {
			method string
			path   string
			code   int
		}{
			{"GET", "/", http.StatusOK},
			{"GET", "/run", http.StatusOK},
			{"GET", "/run/", http.StatusOK},
			{"GET", "/inventory", http.StatusOK},
			{"GET", "/inventory/", http.StatusOK},
			{"GET", "/stats", http.StatusOK},
			{"GET", "/stats/", http.StatusOK},
			{"GET", "/prom", http.StatusNoContent},
			{"GET", "/prom/", http.StatusNoContent},
		}
		for _, reqtest := range reqtests {
			t.Logf("OK path (%s %s)", reqtest.method, reqtest.path)
			req := httptest.NewRequest(reqtest.method, reqtest.path, nil)
			w := httptest.NewRecorder()
			s.router(w, req)
			resp := w.Result()
			if resp.StatusCode != reqtest.code {
				t.Fatalf("expected %d, got %d", reqtest.code, resp.StatusCode)
			}
		}
		viper.Reset()
	}

	t.Log("invalid (PUT /write/foo) w/o data")
	{
		viper.Set(config.KeyStatsdDisabled, true)
		viper.Set(config.KeyPluginDir, "testdata/")
		p := plugins.New(context.Background())
		s, _ := New(p, nil)
		req := httptest.NewRequest("PUT", "/write/foo", nil)
		w := httptest.NewRecorder()
		s.router(w, req)
		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
		}
		viper.Reset()
	}

	t.Log("OK (PUT /write/foo) w/data")
	{
		s, _ := New(nil, nil)
		reqBody := bytes.NewReader([]byte(`{"test":{"_type":"i", "_value":1}}`))
		req := httptest.NewRequest("PUT", "/write/foo", reqBody)
		w := httptest.NewRecorder()
		s.router(w, req)
		resp := w.Result()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected %d, got %d", http.StatusNoContent, resp.StatusCode)
		}
	}
}
