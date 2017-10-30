// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/plugins"
	cgm "github.com/circonus-labs/circonus-gometrics"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestRun(t *testing.T) {
	t.Log("Testing run")
	viper.Reset()
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
	viper.Set(config.KeyListen, ":2609")
	b, berr := builtins.New()
	if berr != nil {
		t.Fatalf("expected no error, got (%s)", berr)
	}
	p, perr := plugins.New(context.Background())
	if perr != nil {
		t.Fatalf("expected NO error, got (%s)", perr)
	}
	if err := p.Scan(b); err != nil {
		t.Fatalf("expected no error, got (%s)", err)
	}

	s, err := New(b, p, nil)
	if err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}
	if s == nil {
		t.Fatal("expected NOT nil")
	}

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

	viper.Reset()
}

func TestInventory(t *testing.T) {
	t.Log("Testing inventory")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("unable to get cwd (%s)", err)
	}
	testDir := path.Join(dir, "testdata")

	viper.Set(config.KeyListen, ":2609")
	viper.Set(config.KeyPluginDir, testDir)
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
	time.Sleep(1 * time.Second)

	t.Logf("GET /inventory -> %d", http.StatusOK)
	req := httptest.NewRequest("GET", "/inventory", nil)
	w := httptest.NewRecorder()

	s.inventory(w, req)

	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, resp.StatusCode)
	}

	viper.Reset()
}

func TestWrite(t *testing.T) {
	t.Log("Testing write")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	viper.Set(config.KeyListen, ":2609")
	s, err := New(nil, nil, nil)
	if err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}
	if s == nil {
		t.Fatal("expected NOT nil")
	}

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

		s.write(w, req)

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

		s.write(w, req)

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

		s.write(w, req)

		resp := w.Result()

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected %d, got %d", http.StatusNoContent, resp.StatusCode)
		}
	}

	viper.Reset()
}

func TestSocketHandler(t *testing.T) {
	t.Log("Testing socketHandler")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	viper.Set(config.KeyListen, ":2609")
	s, err := New(nil, nil, nil)
	if err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}
	if s == nil {
		t.Fatal("expected NOT nil")
	}

	t.Logf("GET /write/ -> %d", http.StatusNotFound)
	{
		req := httptest.NewRequest("GET", "/write/", nil)
		w := httptest.NewRecorder()
		s.socketHandler(w, req)
		resp := w.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected %d, got %d", http.StatusNotFound, resp.StatusCode)
		}
	}

	t.Logf("GET /write/foo -> %d", http.StatusMethodNotAllowed)
	{
		req := httptest.NewRequest("GET", "/write/foo", nil)
		w := httptest.NewRecorder()
		s.socketHandler(w, req)
		resp := w.Result()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("expected %d, got %d", http.StatusMethodNotAllowed, resp.StatusCode)
		}
	}

	t.Logf("PUT /write/foo w/o data -> %d", http.StatusBadRequest)
	{
		req := httptest.NewRequest("PUT", "/write/foo", nil)
		w := httptest.NewRecorder()

		s.socketHandler(w, req)

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

		s.socketHandler(w, req)

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

		s.socketHandler(w, req)

		resp := w.Result()

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected %d, got %d", http.StatusNoContent, resp.StatusCode)
		}
	}

	viper.Reset()
}

func TestPromOutput(t *testing.T) {
	t.Log("Testing promOutput")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	viper.Set(config.KeyListen, ":2609")
	s, err := New(nil, nil, nil)
	if err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}
	if s == nil {
		t.Fatal("expected NOT nil")
	}

	t.Logf("GET /prom -> %d (w/o metrics)", http.StatusNoContent)
	{
		req := httptest.NewRequest("GET", "/prom", nil)
		w := httptest.NewRecorder()

		s.promOutput(w, req)

		resp := w.Result()

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected %d, got %d", http.StatusNoContent, resp.StatusCode)
		}
	}

	t.Logf("GET /prom -> %d (w/metrics)", http.StatusOK)
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

	viper.Reset()
}

func TestMetricsToPromFormat(t *testing.T) {
	t.Log("Testing metricsToPromFormat")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	viper.Set(config.KeyListen, ":2609")
	s, err := New(nil, nil, nil)
	if err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}
	if s == nil {
		t.Fatal("expected NOT nil")
	}
	ts := int64(12345)

	t.Log("basic coverage (*cgm.Metrics -> cgm.Metrics -> cgm.Metric)")
	{
		mgroup := "g"
		mname := "m"
		mtype := "i"
		mval := 1

		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		m := &cgm.Metrics{
			mname: cgm.Metric{Type: mtype, Value: mval},
		}
		s.metricsToPromFormat(w, mgroup, ts, m)
		w.Flush()
		expect := fmt.Sprintf("%s`%s %d %d\n", mgroup, mname, mval, ts)
		if b.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, b.String())
		}
	}

	t.Log("bad int conversion")
	{
		mname := "m"
		mtype := "i"
		mval := "a"

		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		m := cgm.Metric{Type: mtype, Value: mval}
		s.metricsToPromFormat(w, mname, ts, m)
		w.Flush()
		expect := ""
		if b.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, b.String())
		}
	}

	t.Log("simple float")
	{
		mname := "m"
		mtype := "n"
		mval := 3.12

		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		m := cgm.Metric{Type: mtype, Value: mval}
		s.metricsToPromFormat(w, mname, ts, m)
		w.Flush()
		expect := fmt.Sprintf("%s %f %d\n", mname, mval, ts)
		if b.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, b.String())
		}
	}

	t.Log("bad float conversion")
	{
		mname := "m"
		mtype := "n"
		mval := "a"

		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		m := cgm.Metric{Type: mtype, Value: mval}
		s.metricsToPromFormat(w, mname, ts, m)
		w.Flush()
		expect := ""
		if b.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, b.String())
		}
	}

	t.Log("histogram string")
	{
		mname := "m"
		mtype := "n"
		mval := []string{"H[1]=1", "H[2]=1"}

		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		m := cgm.Metric{Type: mtype, Value: mval}
		s.metricsToPromFormat(w, mname, ts, m)
		w.Flush()
		expect := ""
		if b.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, b.String())
		}
	}

	t.Log("simple text")
	{
		mname := "m"
		mtype := "s"
		mval := "foo"

		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		m := cgm.Metric{Type: mtype, Value: mval}
		s.metricsToPromFormat(w, mname, ts, m)
		w.Flush()
		expect := ""
		if b.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, b.String())
		}
	}

	t.Log("invalid metric type")
	{
		mname := "m"
		mtype := "q"
		mval := "bar"

		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		m := cgm.Metric{Type: mtype, Value: mval}
		s.metricsToPromFormat(w, mname, ts, m)
		w.Flush()
		expect := ""
		if b.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, b.String())
		}
	}

	t.Log("unhandled value type")
	{
		mname := "foo"

		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		m := []int{1, 2, 3}
		s.metricsToPromFormat(w, mname, ts, m)
		w.Flush()
		expect := ""
		if b.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, b.String())
		}
	}

	viper.Reset()
}
