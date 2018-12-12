// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package receiver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
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
		r := ioutil.NopCloser(bytes.NewReader(data))
		expectedErr := errors.New("parsing json for test: EOF")
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
		r := ioutil.NopCloser(bytes.NewReader(data))
		expectedErr := errors.New("parsing json for test: EOF")
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
		r := ioutil.NopCloser(bytes.NewReader(data))
		expectedErr := errors.New("parsing json for test: unexpected EOF")
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
		r := ioutil.NopCloser(bytes.NewReader(data))
		expectedErr := errors.New("id:test - offset 10: invalid character '}' looking for beginning of value")
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
		r := ioutil.NopCloser(bytes.NewReader(data))
		err := Parse("test", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		if len(*m) != 0 {
			t.Fatalf("expected no metrics, got %#v", metrics)
		}
	}

	t.Log("\ttype 'i' int32")
	{
		data := []byte(`{"test": {"_type": "i", "_value": 1}}`)
		r := ioutil.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)["testg`test"]
		if !ok {
			t.Fatalf("expected metric 'testg`test', %#v", m)
		}
		if testMetric.Value.(int32) != int32(1) {
			t.Fatalf("expected 1 got %v", testMetric.Value)
		}
	}

	t.Log("\ttype 'I' uint32")
	{
		data := []byte(`{"test": {"_type": "I", "_value": 1}}`)
		r := ioutil.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)["testg`test"]
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
		r := ioutil.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)["testg`test"]
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
		r := ioutil.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)["testg`test"]
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
		r := ioutil.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)["testg`test"]
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
		r := ioutil.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)["testg`test"]
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
		r := ioutil.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)["testg`test"]
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
		r := ioutil.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)["testg`test"]
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
		r := ioutil.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)["testg`test"]
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
		r := ioutil.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)["testg`test"]
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
		r := ioutil.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)["testg`test"]
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
		r := ioutil.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		m := metrics.FlushMetrics()
		testMetric, ok := (*m)["testg`test"]
		if ok {
			t.Fatalf("expected no metric got, %#v", testMetric)
		}
	}

	t.Log("\twith tags")
	{
		data := []byte(`{"test": {"_tags": ["c1:v1","c2:v2"], "_type": "n", "_value": 1}}`)
		r := ioutil.NopCloser(bytes.NewReader(data))
		err := Parse("testg", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		mn := "testg`test" + `|ST[b"YzE=":b"djE=",b"YzI=":b"djI="]`
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
		Description string
		Value       interface{}
		Expect      int32
		ShouldFail  bool
	}{
		{"valid", 1, int32(1), false},
		{"valid, string", fmt.Sprintf("%v", 1), int32(1), false},
		{"bad conversion", fmt.Sprintf("%v", "1a"), 0, true},
		{"bad data type", []int{1}, 0, true},
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
		Description string
		Value       interface{}
		Expect      uint32
		ShouldFail  bool
	}{
		{"valid", 1, uint32(1), false},
		{"valid, string", fmt.Sprintf("%v", 1), uint32(1), false},
		{"bad conversion", fmt.Sprintf("%v", "1a"), 0, true},
		{"bad data type", []int{1}, 0, true},
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
		Description string
		Value       interface{}
		Expect      int64
		ShouldFail  bool
	}{
		{"valid", 1, int64(1), false},
		{"valid, string", fmt.Sprintf("%v", 1), int64(1), false},
		{"bad conversion", fmt.Sprintf("%v", "1a"), 0, true},
		{"bad data type", []int{1}, 0, true},
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
		Description string
		Value       interface{}
		Expect      uint64
		ShouldFail  bool
	}{
		{"valid", 1, uint64(1), false},
		{"valid, string", fmt.Sprintf("%v", 1), uint64(1), false},
		{"bad conversion", fmt.Sprintf("%v", "1a"), 0, true},
		{"bad data type", []int{1}, 0, true},
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
		Description string
		Value       interface{}
		Expect      float64
		ShouldFail  bool
	}{
		{"valid1", 1, float64(1), false},
		{"valid2", 1.2, float64(1.2), false},
		{"valid, string1", fmt.Sprintf("%v", 1), float64(1), false},
		{"valid, string2", fmt.Sprintf("%v", 1.2), float64(1.2), false},
		{"bad conversion", fmt.Sprintf("%v", "1a"), 0, true},
		{"bad data type", true, 0, true},
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
			}
			if fmt.Sprintf("%v", *v) != fmt.Sprintf("%v", test.Expect) {
				t.Fatalf("expected (%#v) got (%#v)", test.Expect, *v)
			}
		}
	}
}
