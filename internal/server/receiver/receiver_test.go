// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package receiver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/tags"
	"github.com/rs/zerolog"
)

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

	t.Log("\tinvalid json (no data)")
	{
		data := []byte{}
		r := io.NopCloser(bytes.NewReader(data))
		expectedErr := fmt.Errorf("parsing json for test: EOF") //nolint:goerr113
		err := Parse("test", r)
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("\tinvalid json (blank)")
	{
		data := []byte("")
		r := io.NopCloser(bytes.NewReader(data))
		expectedErr := fmt.Errorf("parsing json for test: EOF") //nolint:goerr113
		err := Parse("test", r)
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("\tinvalid json (syntax)")
	{
		data := []byte("{")
		r := io.NopCloser(bytes.NewReader(data))
		expectedErr := fmt.Errorf("parsing json for test: unexpected EOF") //nolint:goerr113
		err := Parse("test", r)
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("\tinvalid json (syntax)")
	{
		data := []byte(`{"test": }`)
		r := io.NopCloser(bytes.NewReader(data))
		expectedErr := fmt.Errorf("id:test - offset 10 -- invalid character '}' looking for beginning of value") //nolint:goerr113
		err := Parse("test", r)
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("\tno metrics")
	{
		data := []byte("{}")
		r := io.NopCloser(bytes.NewReader(data))
		err := Parse("test", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		if len(*m) != 0 {
			t.Fatalf("expected no metrics, got %#v", metrics)
		}
	}

	baseTags := tags.Tags{
		tags.Tag{Category: "source", Value: "circonus-agent"},
		tags.Tag{Category: "collector", Value: "write"},
		tags.Tag{Category: "collector_id", Value: "testg"},
	}
	metricName := tags.MetricNameWithStreamTags("test", baseTags)

	t.Log("\ttype 'i' int32")
	{
		data := []byte(`{"test": {"_type": "i", "_value": 1}}`)
		r := io.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)[metricName]
		if !ok {
			t.Fatalf("expected metric '%s', got (%v)", metricName, m)
		}
		if testMetric.Value.(int32) != int32(1) {
			t.Fatalf("expected 1 got %v", testMetric.Value)
		}
	}

	t.Log("\ttype 'I' uint32")
	{
		data := []byte(`{"test": {"_type": "I", "_value": 1}}`)
		r := io.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)[metricName]
		if !ok {
			t.Fatalf("expected metric 'testg`test', %#v", m)
		}
		if testMetric.Value.(uint32) != uint32(1) {
			t.Fatalf("expected 1 got %v", testMetric.Value)
		}
	}

	t.Log("\ttype 'l' int64")
	{
		data := []byte(`{"test": {"_type": "l", "_value": 1}}`)
		r := io.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)[metricName]
		if !ok {
			t.Fatalf("expected metric 'testg`test', %#v", m)
		}
		if testMetric.Value.(int64) != int64(1) {
			t.Fatalf("expected 1 got %v", testMetric.Value)
		}
	}

	t.Log("\ttype 'L' uint64")
	{
		data := []byte(`{"test": {"_type": "L", "_value": 1}}`)
		r := io.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)[metricName]
		if !ok {
			t.Fatalf("expected metric 'testg`test', %#v", m)
		}
		if testMetric.Value.(uint64) != uint64(1) {
			t.Fatalf("expected 1 got %v", testMetric.Value)
		}
	}

	t.Log("\ttype 'n' float")
	{
		data := []byte(`{"test": {"_type": "n", "_value": 1}}`)
		r := io.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)[metricName]
		if !ok {
			t.Fatalf("expected metric 'testg`test', %#v", m)
		}
		if testMetric.Value.(float64) != float64(1) {
			t.Fatalf("expected 1 got %v", testMetric.Value)
		}
	}

	t.Log("\ttype 'n' float (histogram numeric samples)")
	{
		data := []byte(`{"test": {"_type": "n", "_value": [1]}}`)
		r := io.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)[metricName]
		if !ok {
			t.Fatalf("expected metric 'testg`test', %#v", m)
		}
		if len(testMetric.Value.([]string)) == 0 {
			t.Fatalf("expected at least 1 sample, got %#v", testMetric.Value)
		}
		expect := "[H[1.0e+00]=1]"
		if !strings.Contains(fmt.Sprintf("%v", testMetric.Value), expect) {
			t.Fatalf("expected (%v) got (%v)", expect, testMetric.Value)
		}
	}

	t.Log("\ttype 'n' float (histogram encoded samples)")
	{
		data := []byte(`{"test": {"_type": "n", "_value": ["H[1.2]=1"]}}`)
		r := io.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)[metricName]
		if !ok {
			t.Fatalf("expected metric 'testg`test', %#v", m)
		}
		if len(testMetric.Value.([]string)) == 0 {
			t.Fatalf("expected at least 1 sample, got %#v", testMetric.Value)
		}
		expect := "[H[1.2e+00]=1]"
		if !strings.Contains(fmt.Sprintf("%v", testMetric.Value), expect) {
			t.Fatalf("expected (%v) got (%v)", expect, testMetric.Value)
		}
	}

	t.Log("\ttype 'h' float")
	{
		data := []byte(`{"test": {"_type": "h", "_value": 1}}`)
		r := io.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)[metricName]
		if !ok {
			t.Fatalf("expected metric 'testg`test', %#v", m)
		}
		if testMetric.Value.(float64) != float64(1) {
			t.Fatalf("expected 1 got %v", testMetric.Value)
		}
	}

	t.Log("\ttype 'h' float (histogram numeric samples)")
	{
		data := []byte(`{"test": {"_type": "h", "_value": [1]}}`)
		r := io.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)[metricName]
		if !ok {
			t.Fatalf("expected metric 'testg`test', %#v", m)
		}
		if len(testMetric.Value.([]string)) == 0 {
			t.Fatalf("expected at least 1 sample, got %#v", testMetric.Value)
		}
		expect := "[H[1.0e+00]=1]"
		if !strings.Contains(fmt.Sprintf("%v", testMetric.Value), expect) {
			t.Fatalf("expected (%v) got (%v)", expect, testMetric.Value)
		}
	}

	t.Log("\ttype 'h' float (histogram encoded samples)")
	{
		data := []byte(`{"test": {"_type": "h", "_value": ["H[1.2]=1"]}}`)
		r := io.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)[metricName]
		if !ok {
			t.Fatalf("expected metric 'testg`test', %#v", m)
		}
		if len(testMetric.Value.([]string)) == 0 {
			t.Fatalf("expected at least 1 sample, got %#v", testMetric.Value)
		}
		expect := "[H[1.2e+00]=1]"
		if !strings.Contains(fmt.Sprintf("%v", testMetric.Value), expect) {
			t.Fatalf("expected (%v) got (%v)", expect, testMetric.Value)
		}
	}

	t.Log("\ttype 's' string")
	{
		data := []byte(`{"test": {"_type": "s", "_value": "foo"}}`)
		r := io.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)[metricName]
		if !ok {
			t.Fatalf("expected metric 'testg`test', %#v", m)
		}
		if testMetric.Value.(string) != "foo" {
			t.Fatalf("expected 'foo' got '%v'", testMetric.Value)
		}
	}

	t.Log("\ttype 'z' invalid type")
	{
		data := []byte(`{"test": {"_type": "z", "_value": null}}`)
		r := io.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)[metricName]
		if ok {
			t.Fatalf("expected no metric got, %#v", testMetric)
		}
	}

	t.Log("\twith tags")
	{
		data := []byte(`{"test": {"_tags": ["c1:v1","c2:v2"], "_type": "n", "_value": 1}}`)
		r := io.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		var tagList tags.Tags
		tagList = append(tagList, baseTags...)
		tagList = append(tagList, tags.Tags{
			tags.Tag{Category: "c1", Value: "v1"},
			tags.Tag{Category: "c2", Value: "v2"},
		}...)
		mn := tags.MetricNameWithStreamTags("test", tagList)
		m := metrics.FlushMetrics()
		_, ok := (*m)[mn]
		if !ok {
			t.Fatalf("expected metric '%s', %#v", mn, m)
		}
	}

}

func createMetric(t string, v interface{}) tags.JSONMetric {

	// convert native literal types to json then back to
	// simulate parsed values coming in from a POST|PUT
	// iow, what would be _coming_ from receive.Parse()

	m := tags.JSONMetric{Type: t, Value: v}
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}

	if err := json.Unmarshal(b, &m); err != nil {
		panic(err)
	}
	return m
}

func TestParseInt32(t *testing.T) {
	t.Log("Testing parseInt32")

	metricType := "i"

	tt := []struct {
		Value       interface{}
		Description string
		Expect      int32
		ShouldFail  bool
	}{
		{Description: "valid", Value: 1, Expect: int32(1), ShouldFail: false},
		{Description: "valid, string", Value: fmt.Sprintf("%v", 1), Expect: int32(1), ShouldFail: false},
		{Description: "bad conversion", Value: fmt.Sprintf("%v", "1a"), Expect: 0, ShouldFail: true},
		{Description: "bad data type", Value: []int{1}, Expect: 0, ShouldFail: true},
	}

	for _, test := range tt {
		t.Logf("\ttesting %s (%#v)", test.Description, test.Value)
		metric := createMetric(metricType, test.Value)
		v := parseInt32("test", metric)
		if test.ShouldFail {
			if v != nil {
				t.Fatalf("expected nil, got (%#v)", v)
			}
		} else {
			if v == nil {
				t.Fatal("expected value")
				return
			}
			if *v != test.Expect {
				t.Fatalf("expected (%#v) got (%#v)", test.Expect, *v)
			}
		}
	}
}

func TestParseUint32(t *testing.T) {
	t.Log("Testing parseUint32")

	metricType := "I"

	tt := []struct {
		Value       interface{}
		Description string
		Expect      uint32
		ShouldFail  bool
	}{
		{Description: "valid", Value: 1, Expect: uint32(1), ShouldFail: false},
		{Description: "valid, string", Value: fmt.Sprintf("%v", 1), Expect: uint32(1), ShouldFail: false},
		{Description: "bad conversion", Value: fmt.Sprintf("%v", "1a"), Expect: 0, ShouldFail: true},
		{Description: "bad data type", Value: []int{1}, Expect: 0, ShouldFail: true},
	}

	for _, test := range tt {
		t.Logf("\ttesting %s (%#v)", test.Description, test.Value)
		metric := createMetric(metricType, test.Value)
		v := parseUint32("test", metric)
		if test.ShouldFail {
			if v != nil {
				t.Fatalf("expected nil, got (%#v)", v)
			}
		} else {
			if v == nil {
				t.Fatal("expected value")
				return
			}
			if *v != test.Expect {
				t.Fatalf("expected (%#v) got (%#v)", test.Expect, *v)
			}
		}
	}
}

func TestParseInt64(t *testing.T) {
	t.Log("Testing parseInt64")

	metricType := "l"

	tt := []struct {
		Value       interface{}
		Description string
		Expect      int64
		ShouldFail  bool
	}{
		{Description: "valid", Value: 1, Expect: int64(1), ShouldFail: false},
		{Description: "valid, string", Value: fmt.Sprintf("%v", 1), Expect: int64(1), ShouldFail: false},
		{Description: "bad conversion", Value: fmt.Sprintf("%v", "1a"), Expect: 0, ShouldFail: true},
		{Description: "bad data type", Value: []int{1}, Expect: 0, ShouldFail: true},
	}

	for _, test := range tt {
		t.Logf("\ttesting %s (%#v)", test.Description, test.Value)
		metric := createMetric(metricType, test.Value)
		v := parseInt64("test", metric)
		if test.ShouldFail {
			if v != nil {
				t.Fatalf("expected nil, got (%#v)", v)
			}
		} else {
			if v == nil {
				t.Fatal("expected value")
				return
			}
			if *v != test.Expect {
				t.Fatalf("expected (%#v) got (%#v)", test.Expect, *v)
			}
		}
	}
}

func TestParseUint64(t *testing.T) {
	t.Log("Testing parseUint64")

	metricType := "L"

	tt := []struct {
		Value       interface{}
		Description string
		Expect      uint64
		ShouldFail  bool
	}{
		{Description: "valid", Value: 1, Expect: uint64(1), ShouldFail: false},
		{Description: "valid, string", Value: fmt.Sprintf("%v", 1), Expect: uint64(1), ShouldFail: false},
		{Description: "bad conversion", Value: fmt.Sprintf("%v", "1a"), Expect: 0, ShouldFail: true},
		{Description: "bad data type", Value: []int{1}, Expect: 0, ShouldFail: true},
	}

	for _, test := range tt {
		t.Logf("\ttesting %s (%#v)", test.Description, test.Value)
		metric := createMetric(metricType, test.Value)
		v := parseUint64("test", metric)
		if test.ShouldFail {
			if v != nil {
				t.Fatalf("expected nil, got (%#v)", v)
			}
		} else {
			if v == nil {
				t.Fatal("expected value")
				return
			}
			if *v != test.Expect {
				t.Fatalf("expected (%#v) got (%#v)", test.Expect, *v)
			}
		}
	}
}

func TestParseFloat(t *testing.T) {
	t.Log("Testing parseFloat")

	metricType := "n"

	tt := []struct {
		Value       interface{}
		Description string
		Expect      float64
		ShouldFail  bool
	}{
		{Description: "valid1", Value: 1, Expect: float64(1), ShouldFail: false},
		{Description: "valid2", Value: 1.2, Expect: float64(1.2), ShouldFail: false},
		{Description: "valid, string1", Value: fmt.Sprintf("%v", 1), Expect: float64(1), ShouldFail: false},
		{Description: "valid, string2", Value: fmt.Sprintf("%v", 1.2), Expect: float64(1.2), ShouldFail: false},
		{Description: "bad conversion", Value: fmt.Sprintf("%v", "1a"), Expect: 0, ShouldFail: true},
		{Description: "bad data type", Value: true, Expect: 0, ShouldFail: true},
	}

	for _, test := range tt {
		t.Logf("\ttesting %s (%#v)", test.Description, test.Value)
		metric := createMetric(metricType, test.Value)
		v, isHist := parseFloat("test", metric)
		if isHist {
			t.Fatal("not expecting histogram")
		}
		if test.ShouldFail {
			if v != nil {
				t.Fatalf("expected nil, got (%#v)", v)
			}
		} else {
			if v == nil {
				t.Fatal("expected value")
				return
			}
			if *v != test.Expect {
				t.Fatalf("expected (%#v) got (%#v)", test.Expect, *v)
			}
		}
	}
}

func TestParseHistogram(t *testing.T) {
	t.Log("Testing parseHistogram")

	metricType := "n"

	tt := []struct {
		Description string
		Value       interface{}
		Expect      []histSample
		ShouldFail  bool
	}{
		{"valid1", []float64{1}, []histSample{{bucket: false, count: 0, value: 1}}, false},
		{"valid2", []float64{1.2}, []histSample{{bucket: false, count: 0, value: 1.2}}, false},
		{"valid hist", []string{"H[1.2]=1"}, []histSample{{bucket: true, count: 1, value: 1.2}}, false},
		{"valid, string1", []string{fmt.Sprintf("%v", 1)}, []histSample{{bucket: false, count: 0, value: 1}}, false},
		{"valid, string2", []string{fmt.Sprintf("%v", 1.2)}, []histSample{{bucket: false, count: 0, value: 1.2}}, false},
		{"bad conversion", []string{fmt.Sprintf("%v", "1a")}, []histSample{}, true},
		{"bad data type - metric", true, []histSample{}, true},
		{"bad data type - metric sample", []bool{true}, []histSample{}, true},
		{"bad hist val", []string{"H[1.2b]=1"}, []histSample{}, true},
		{"bad hist cnt", []string{"H[1.2]=1b"}, []histSample{}, true},
	}

	for _, test := range tt {
		t.Logf("\ttesting %s (%#v)", test.Description, test.Value)
		metric := createMetric(metricType, test.Value)
		v := parseHistogram("test", metric)
		if test.ShouldFail {
			if v != nil {
				t.Fatalf("expected nil, got (%#v)", v)
			}
		} else {
			if v == nil {
				t.Fatal("expected value")
				return
			}
			if fmt.Sprintf("%v", *v) != fmt.Sprintf("%v", test.Expect) {
				t.Fatalf("expected (%#v) got (%#v)", test.Expect, *v)
			}
		}
	}
}
