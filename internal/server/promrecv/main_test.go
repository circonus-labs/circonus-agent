// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package promrecv

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"testing"

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

func TestFlush(t *testing.T) {
	t.Log("Testing Flush")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("\tno metrics")
	{
		m := Flush()
		if len(*m) != 0 {
			t.Fatalf("expected 0 metrics, got %d", len(*m))
		}
	}

	t.Log("\tw/metric(s)")
	{
		err := initCGM()
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
		metrics.SetText("test", "test")

		m := Flush()
		if len(*m) != 1 {
			t.Fatalf("expected 1 metric, got %d", len(*m))
		}
	}
}

func TestParse(t *testing.T) {
	t.Log("Testing Parse")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	err := initCGM()
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}

	t.Log("\tno data")
	{
		data := []byte{}
		r := ioutil.NopCloser(bytes.NewReader(data))
		err := Parse(r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("\tblank/empty data")
	{
		data := []byte("")
		r := ioutil.NopCloser(bytes.NewReader(data))
		err := Parse(r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("\tvalid data")
	{
		data := []byte(promData)
		r := ioutil.NopCloser(bytes.NewReader(data))
		err := Parse(r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		numExpected := 22
		if len(*m) != numExpected {
			t.Fatalf("expected %d metrics, got %d", numExpected, len(*m))
		}

		// test 1
		{
			mn := "prom`test"
			testMetric, ok := (*m)[mn]
			if !ok {
				t.Fatalf("expected metric '%s', %#v", mn, m)
			}
			expect := float64(1234)
			if testMetric.Value.(float64) != expect {
				t.Fatalf("expected %v got %v", expect, testMetric.Value)
			}
		}

		// test 2
		{
			// http_requests_total{method="post",code="400"}
			mn := fmt.Sprintf(`%s|ST[b"%s":b"%s",b"%s":b"%s"]`,
				"prom`http_requests_total",
				base64.StdEncoding.EncodeToString([]byte("code")),
				base64.StdEncoding.EncodeToString([]byte("400")),
				base64.StdEncoding.EncodeToString([]byte("method")),
				base64.StdEncoding.EncodeToString([]byte("post")))

			testMetric, ok := (*m)[mn]
			if !ok {
				t.Fatalf("expected metric '%s', %#v", mn, m)
			}
			expect := float64(3)
			if testMetric.Value.(float64) != expect {
				t.Fatalf("expected %v got %v", expect, testMetric.Value)
			}
		}
	}
}
