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

	"github.com/circonus-labs/circonus-agent/internal/builtins"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/plugins"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestRouter(t *testing.T) {
	t.Log("Testing router")
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
		viper.Set(config.KeyListen, ":2609")
		s, err := New(nil, nil, nil)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if s == nil {
			t.Fatal("expected NOT nil")
		}

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
		viper.Set(config.KeyListen, ":2609")
		viper.Set(config.KeyPluginDir, "testdata/")
		p, perr := plugins.New(context.Background())
		if perr != nil {
			t.Fatalf("expected NO error, got (%s)", perr)
		}
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
		s, err := New(nil, p, nil)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if s == nil {
			t.Fatal("expected NOT nil")
		}
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
		viper.Set(config.KeyListen, ":2609")
		viper.Set(config.KeyStatsdDisabled, true)
		viper.Set(config.KeyPluginDir, "testdata/")
		b, berr := builtins.New()
		if berr != nil {
			t.Fatalf("expected no error, got (%s)", berr)
		}
		p, perr := plugins.New(context.Background())
		if perr != nil {
			t.Fatalf("expected NO error, got (%s)", perr)
		}
		s, err := New(b, p, nil)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if s == nil {
			t.Fatal("expected NOT nil")
		}
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
		viper.Set(config.KeyListen, ":2609")
		viper.Set(config.KeyStatsdDisabled, true)
		viper.Set(config.KeyPluginDir, "testdata/")
		p, perr := plugins.New(context.Background())
		if perr != nil {
			t.Fatalf("expected NO error, got (%s)", perr)
		}
		s, err := New(nil, p, nil)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if s == nil {
			t.Fatal("expected NOT nil")
		}

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
		viper.Set(config.KeyListen, ":2609")
		s, err := New(nil, nil, nil)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if s == nil {
			t.Fatal("expected NOT nil")
		}
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
