// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package receiver

import (
	"bytes"
	"errors"
	"io/ioutil"
	"testing"

	"github.com/rs/zerolog"
)

func TestFlush(t *testing.T) {
	t.Log("Testing Flush")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("No metrics")
	{
		m := Flush()
		if len(*m) != 0 {
			t.Fatalf("expected 0 metrics, got %d", len(*m))
		}
	}

	metrics = &Metrics{"test": "test"}

	t.Log("Valid w/1 metric")
	{
		m := Flush()
		if len(*m) != 1 {
			t.Fatalf("expected 1 metric, got %d", len(*m))
		}
	}
}

func TestNumMetricGroups(t *testing.T) {
	t.Log("Testing NumMetricGroups")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("No metrics")
	{
		n := NumMetricGroups()
		if n != 0 {
			t.Fatalf("expected 0 metrics, got %d", n)
		}
	}

	metrics = &Metrics{"test": "test"}

	t.Log("Valid w/1 metric")
	{
		n := NumMetricGroups()
		if n != 1 {
			t.Fatalf("expected 0 metrics, got %d", n)
		}
	}

}

func TestParse(t *testing.T) {
	t.Log("Testing Parse")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("invalid json (no data)")
	{
		metrics = nil
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

	t.Log("invalid json (blank)")
	{
		metrics = nil
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

	t.Log("invalid json (syntax)")
	{
		metrics = nil
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

	t.Log("invalid json (syntax)")
	{
		metrics = nil
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

	t.Log("no metrics")
	{
		metrics = nil
		data := []byte("{}")
		r := ioutil.NopCloser(bytes.NewReader(data))
		err := Parse("test", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len((*metrics)["test"].(map[string]interface{})) != 0 {
			t.Fatalf("expected no metrics, got %#v", metrics)
		}
	}

	t.Log("Valid w/1 metric")
	{
		data := []byte(`{"metric": 1}`)
		r := ioutil.NopCloser(bytes.NewReader(data))
		err := Parse("test", r)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		mg, ok := (*metrics)["test"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected metric group 'test', %#v", metrics)
		}
		m, ok := mg["metric"]
		if !ok {
			t.Fatalf("expected metric 'metric', %#v", mg)
		}
		if m.(float64) != float64(1) {
			t.Fatalf("expected 1 got %v", m)
		}
	}
}
