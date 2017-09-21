// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/plugins"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestRun(t *testing.T) {
	t.Log("Testing run")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	runTests := []struct {
		path string
		code int
	}{
		{"/run/foo", http.StatusNotFound},
		{"/", http.StatusOK},
		{"/run", http.StatusOK},
		{"/run/test", http.StatusOK},
		{"/run/write", http.StatusOK},
		{"/run/statsd", http.StatusOK},
	}

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("unable to get cwd (%s)", err)
	}
	testDir := path.Join(dir, "testdata")

	viper.Set(config.KeyPluginDir, testDir)
	plugins.Initialize()

	for _, runReq := range runTests {
		time.Sleep(1 * time.Second)
		t.Logf("GET %s -> %d", runReq.path, runReq.code)
		req := httptest.NewRequest("GET", runReq.path, nil)
		w := httptest.NewRecorder()

		run(w, req)

		resp := w.Result()

		if resp.StatusCode != runReq.code {
			t.Fatalf("expected %d, got %d", runReq.code, resp.StatusCode)
		}

	}
}

func TestInventory(t *testing.T) {
	t.Log("Testing inventory")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("unable to get cwd (%s)", err)
	}
	testDir := path.Join(dir, "testdata")

	viper.Set(config.KeyPluginDir, testDir)
	plugins.Initialize()
	time.Sleep(1 * time.Second)

	t.Log("GET /inventory -> 200")
	req := httptest.NewRequest("GET", "/inventory", nil)
	w := httptest.NewRecorder()

	inventory(w, req)

	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestWrite(t *testing.T) {
	t.Log("Testing write")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("PUT /write/ -> 404")
	{
		req := httptest.NewRequest("GET", "/write/", nil)
		w := httptest.NewRecorder()

		write(w, req)

		resp := w.Result()

		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected %d, got %d", http.StatusNotFound, resp.StatusCode)
		}
	}

	t.Log("PUT /write/foo w/o data -> 500")
	{
		req := httptest.NewRequest("PUT", "/write/foo", nil)
		w := httptest.NewRecorder()

		router(w, req)

		resp := w.Result()

		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected %d, got %d", http.StatusInternalServerError, resp.StatusCode)
		}
	}

	t.Log("PUT /write/foo w/bad data -> 500")
	{
		reqBody := bytes.NewReader([]byte(`{"test":1`))

		req := httptest.NewRequest("PUT", "/write/foo", reqBody)
		w := httptest.NewRecorder()

		router(w, req)

		resp := w.Result()

		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected %d, got %d", http.StatusInternalServerError, resp.StatusCode)
		}
	}

	t.Log("PUT /write/foo w/data -> 204")
	{
		reqBody := bytes.NewReader([]byte(`{"test":1}`))

		req := httptest.NewRequest("PUT", "/write/foo", reqBody)
		w := httptest.NewRecorder()

		router(w, req)

		resp := w.Result()

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected %d, got %d", http.StatusNoContent, resp.StatusCode)
		}
	}

}
