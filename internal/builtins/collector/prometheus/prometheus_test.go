// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package prometheus

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// prometheus exposition formats example from: https://prometheus.io/docs/instrumenting/exposition_formats/
var promData = `
# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 1027 1395066363000
http_requests_total{method="post",code="400"}    3 1395066363000

# Escaping in label values:
msdos_file_access_time_seconds{path="C:\\DIR\\FILE.TXT",error="Cannot find file:\n\"FILE.TXT\""} 1.458255915e9

# Minimalistic line:
metric_without_timestamp_and_labels 12.47

# A weird metric from before the epoch:
something_weird{problem="division by zero"} +Inf -3982045

# A histogram, which has a pretty complex representation in the text format:
# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.05"} 24054
http_request_duration_seconds_bucket{le="0.1"} 33444
http_request_duration_seconds_bucket{le="0.2"} 100392
http_request_duration_seconds_bucket{le="0.5"} 129389
http_request_duration_seconds_bucket{le="1"} 133988
http_request_duration_seconds_bucket{le="+Inf"} 144320
http_request_duration_seconds_sum 53423
http_request_duration_seconds_count 144320

# Finally a summary, which has a complex representation, too:
# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{quantile="0.01"} 3102
rpc_duration_seconds{quantile="0.05"} 3272
rpc_duration_seconds{quantile="0.5"} 4773
rpc_duration_seconds{quantile="0.9"} 9001
rpc_duration_seconds{quantile="0.99"} 76656
rpc_duration_seconds_sum 1.7560473e+07
rpc_duration_seconds_count 2693

# HELP threads_started The number of threads started
# TYPE threads_started gauge
threads_started 3851.0

# test marker
test 1234
`

func TestNew(t *testing.T) {
	t.Log("Testing New")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("no config spec (force default)")
	{
		_, err := New("")
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("missing config file")
	{
		_, err := New(path.Join("testdata", "missing"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("empty config file")
	{
		_, err := New(path.Join("testdata", "empty"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("no prom urls")
	{
		_, err := New(path.Join("testdata", "no_urls"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (include regex)")
	{
		c, err := New(filepath.Join("testdata", "config_include_regex_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := fmt.Sprintf(regexPat, `^foo`)
		if c.(*Prom).include.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, c.(*Prom).include.String())
		}
	}

	t.Log("config (include regex invalid)")
	{
		_, err := New(filepath.Join("testdata", "config_include_regex_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (exclude regex)")
	{
		c, err := New(filepath.Join("testdata", "config_exclude_regex_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		expect := fmt.Sprintf(regexPat, `^foo`)
		if c.(*Prom).exclude.String() != expect {
			t.Fatalf("expected (%s) got (%s)", expect, c.(*Prom).exclude.String())
		}
	}

	t.Log("config (exclude regex invalid)")
	{
		_, err := New(filepath.Join("testdata", "config_exclude_regex_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (metrics enabled setting)")
	{
		c, err := New(filepath.Join("testdata", "config_metrics_enabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len(c.(*Prom).metricStatus) == 0 {
			t.Fatalf("expected >0 metric status settings, got (%#v)", c.(*Prom).metricStatus)
		}
		enabled, ok := c.(*Prom).metricStatus["foo"]
		if !ok {
			t.Fatalf("expected 'foo' key in metric status settings, got (%#v)", c.(*Prom).metricStatus)
		}
		if !enabled {
			t.Fatalf("expected 'foo' to be enabled in metric status settings, got (%#v)", c.(*Prom).metricStatus)
		}
	}

	t.Log("config (metrics disabled setting)")
	{
		c, err := New(filepath.Join("testdata", "config_metrics_disabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len(c.(*Prom).metricStatus) == 0 {
			t.Fatalf("expected >0 metric status settings, got (%#v)", c.(*Prom).metricStatus)
		}
		enabled, ok := c.(*Prom).metricStatus["foo"]
		if !ok {
			t.Fatalf("expected 'foo' key in metric status settings, got (%#v)", c.(*Prom).metricStatus)
		}
		if enabled {
			t.Fatalf("expected 'foo' to be disabled in metric status settings, got (%#v)", c.(*Prom).metricStatus)
		}
	}

	t.Log("config (metrics default status enabled)")
	{
		c, err := New(filepath.Join("testdata", "config_metrics_default_status_enabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if !c.(*Prom).metricDefaultActive {
			t.Fatal("expected true")
		}
	}

	t.Log("config (metrics default status disabled)")
	{
		c, err := New(filepath.Join("testdata", "config_metrics_default_status_disabled_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Prom).metricDefaultActive {
			t.Fatal("expected false")
		}
	}

	t.Log("config (metrics default status invalid)")
	{
		_, err := New(filepath.Join("testdata", "config_metrics_default_status_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("config (run ttl 5m)")
	{
		c, err := New(filepath.Join("testdata", "config_run_ttl_valid_setting"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if c.(*Prom).runTTL != 5*time.Minute {
			t.Fatal("expected 5m")
		}
	}

	t.Log("config (run ttl invalid)")
	{
		_, err := New(filepath.Join("testdata", "config_run_ttl_invalid_setting"))
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("valid")
	{
		c, err := New(path.Join("testdata", "valid"))
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len(c.(*Prom).urls) != 2 {
			t.Fatalf("expected 2 URLs, got (%#v)", c.(*Prom).urls)
		}
	}

}

func TestCollect(t *testing.T) {
	t.Log("Testing Collect")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, promData)
	}))
	defer ts.Close()

	c, err := New(path.Join("testdata", "valid"))
	if err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}
	c.(*Prom).urls = []URLDef{{ID: "foo", URL: ts.URL}}

	if err := c.Collect(); err != nil {
		t.Fatalf("expected no error, got (%s)", err)
	}

	m := c.Flush()
	numExpected := 22
	if len(m) != numExpected {
		t.Fatalf("expected %d metrics, got %d", numExpected, len(m))
	}

	{
		mn := "foo`test"
		testMetric, ok := m[mn]
		if !ok {
			t.Fatalf("expected metric '%s', %#v", mn, m)
		}
		expect := float64(1234)
		if testMetric.Value.(float64) != expect {
			t.Fatalf("expected %v got %v", expect, testMetric.Value)
		}
	}

	{
		// http_requests_total{method="post",code="400"}
		mn := fmt.Sprintf(`%s|ST[b"%s":b"%s",b"%s":b"%s"]`,
			"foo`http_requests_total",
			base64.StdEncoding.EncodeToString([]byte("code")),
			base64.StdEncoding.EncodeToString([]byte("400")),
			base64.StdEncoding.EncodeToString([]byte("method")),
			base64.StdEncoding.EncodeToString([]byte("post")))
		testMetric, ok := m[mn]
		if !ok {
			t.Fatalf("expected metric '%s', %#v", mn, m)
		}
		expect := float64(3)
		if testMetric.Value.(float64) != expect {
			t.Fatalf("expected %v got %v", expect, testMetric.Value)
		}
	}

}

func TestCollectTimeout(t *testing.T) {
	t.Log("Testing Collect w/timeout")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	//
	// collection timing out should be benign
	// return 0 metrics, not throw or cause an error
	// the fact that the timeout was exceeded is logged
	//

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second)
		fmt.Fprintf(w, promData)
	}))
	defer ts.Close()

	c, err := New(path.Join("testdata", "valid"))
	if err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}
	c.(*Prom).urls = []URLDef{{ID: "foo", URL: ts.URL, uttl: 10 * time.Millisecond}}

	if err := c.Collect(); err != nil {
		t.Fatalf("expected no error, got (%s)", err)
	}

	m := c.Flush()
	numExpected := 0
	if len(m) != numExpected {
		t.Fatalf("expected %d metrics, got %d", numExpected, len(m))
	}
}
