// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"bufio"
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/plugins"
	cgm "github.com/circonus-labs/circonus-gometrics"
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
	p := plugins.New(context.Background())
	if err := p.Scan(); err != nil {
		t.Fatalf("expected no error, got (%s)", err)
	}
	s, _ := New(p, nil)

	for _, runReq := range runTests {
		time.Sleep(1 * time.Second)
		t.Logf("GET %s -> %d", runReq.path, runReq.code)
		req := httptest.NewRequest("GET", runReq.path, nil)
		w := httptest.NewRecorder()

		s.run(w, req)

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
	p := plugins.New(context.Background())
	s, _ := New(p, nil)
	time.Sleep(1 * time.Second)

	t.Logf("GET /inventory -> %d", http.StatusOK)
	req := httptest.NewRequest("GET", "/inventory", nil)
	w := httptest.NewRecorder()

	s.inventory(w, req)

	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestWrite(t *testing.T) {
	t.Log("Testing write")

	zerolog.SetGlobalLevel(zerolog.Disabled)
	s, _ := New(nil, nil)

	t.Logf("GET /write/ -> %d", http.StatusNotFound)
	{
		req := httptest.NewRequest("GET", "/write/", nil)
		w := httptest.NewRecorder()

		s.write(w, req)

		resp := w.Result()

		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected %d, got %d", http.StatusNotFound, resp.StatusCode)
		}
	}

	t.Logf("PUT /write/foo w/o data -> %d", http.StatusBadRequest)
	{
		req := httptest.NewRequest("PUT", "/write/foo", nil)
		w := httptest.NewRecorder()

		s.router(w, req)

		resp := w.Result()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
		}
	}

	t.Logf("PUT /write/foo w/bad data -> %d", http.StatusBadRequest)
	{
		reqBody := bytes.NewReader([]byte(`{"test":1`))

		req := httptest.NewRequest("PUT", "/write/foo", reqBody)
		w := httptest.NewRecorder()

		s.router(w, req)

		resp := w.Result()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
		}
	}

	t.Logf("PUT /write/foo w/data -> %d", http.StatusNoContent)
	{
		reqBody := bytes.NewReader([]byte(`{"test":{"_type": "i", "_value":1}}`))

		req := httptest.NewRequest("PUT", "/write/foo", reqBody)
		w := httptest.NewRecorder()

		s.router(w, req)

		resp := w.Result()

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected %d, got %d", http.StatusNoContent, resp.StatusCode)
		}
	}

}

func TestPromOutput(t *testing.T) {
	t.Log("Testing promOutput")

	zerolog.SetGlobalLevel(zerolog.Disabled)
	s, _ := New(nil, nil)

	t.Logf("GET /prom -> %d", http.StatusNoContent)
	{
		req := httptest.NewRequest("GET", "/prom", nil)
		w := httptest.NewRecorder()

		s.promOutput(w, req)

		resp := w.Result()

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected %d, got %d", http.StatusNoContent, resp.StatusCode)
		}
	}

	t.Logf("GET /prom -> %d", http.StatusOK)
	{
		lastMetrics.ts = time.Now()
		lastMetrics.metrics = map[string]interface{}{
			"gtest": &cgm.Metrics{
				"mtest": cgm.Metric{Type: "i", Value: 1},
			},
		}
		req := httptest.NewRequest("GET", "/prom", nil)
		w := httptest.NewRecorder()

		s.promOutput(w, req)

		resp := w.Result()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected %d, got %d", http.StatusOK, resp.StatusCode)
		}

		expect := "gtest`mtest 1"
		body, _ := ioutil.ReadAll(resp.Body)
		if !strings.Contains(string(body), expect) {
			t.Fatalf("expected (%s) got (%s)", expect, string(body))
		}
	}
}

func TestMetricsToPromFormat(t *testing.T) {
	t.Log("Testing metricsToPromFormat")

	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	s, _ := New(nil, nil)

	t.Log("basic coverage (*cgm.Metrics -> cgm.Metrics -> cgm.Metric)")
	{
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		ts := time.Now().UnixNano() / int64(time.Millisecond)
		m := &cgm.Metrics{
			"mtest": cgm.Metric{Type: "i", Value: 1},
		}
		s.metricsToPromFormat(w, "gtest", ts, m)
		w.Flush()
		expect := "gtest`mtest 1"
		if !strings.Contains(b.String(), expect) {
			t.Fatalf("expected (%s) got (%s)", expect, b.String())
		}
	}

	t.Log("bad int conversion")
	{
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		ts := time.Now().UnixNano() / int64(time.Millisecond)
		m := cgm.Metric{Type: "i", Value: "b"}
		s.metricsToPromFormat(w, "mtest", ts, m)
		w.Flush()
		expect := ""
		if b.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, b.String())
		}
	}

	t.Log("simple float")
	{
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		ts := time.Now().UnixNano() / int64(time.Millisecond)
		m := cgm.Metric{Type: "n", Value: 3.12}
		s.metricsToPromFormat(w, "mtest", ts, m)
		w.Flush()
		expect := "mtest 3.12"
		if !strings.Contains(b.String(), expect) {
			t.Fatalf("expected (%s) got (%s)", expect, b.String())
		}
	}

}
