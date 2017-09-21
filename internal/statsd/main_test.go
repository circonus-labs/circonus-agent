// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package statsd

import (
	"errors"
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestFlush(t *testing.T) {
	t.Log("Testing Flush")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Disabled")
	{
		viper.Set(config.KeyStatsdDisabled, true)
		err := Start()
		viper.Reset()

		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("Flush (disabled)")
	{
		viper.Set(config.KeyStatsdDisabled, true)
		metrics := Flush()
		viper.Reset()

		if metrics != nil {
			t.Fatalf("expected nil, got (%#v)", metrics)
		}
	}

	t.Log("Flush (no stats)")
	{
		metrics := Flush()

		if metrics == nil {
			t.Fatal("expected not nil")
		}
		if len(*metrics) != 0 {
			t.Fatalf("expected empty metrics, got (%#v)", metrics)
		}
	}
}

func TestGetMetricDest(t *testing.T) {
	t.Log("Testing getMetricDest")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Defaults")
	{
		// if no prefix for host or group set, all metrics go to host
		dtest := []struct {
			metricName   string
			expectedDest string
			expectedName string
		}{
			{"host.foo", "host", "host.foo"},
			{"group.foo", "host", "group.foo"},
			{"foo", "host", "foo"},
		}
		cfg := initSettings()
		for _, d := range dtest {
			t.Logf("%s -> %s", d.metricName, d.expectedDest)
			dest, metric := getMetricDestination(cfg, d.metricName)
			if dest != d.expectedDest {
				t.Fatalf("dest expected '%s' got '%s'", d.expectedDest, dest)
			}
			if metric != d.expectedName {
				t.Fatalf("name expected '%s' got '%s'", d.metricName, metric)
			}
		}
	}

	t.Log("Explicit 'host.' & 'group.'")
	{
		// both group and host metrics have a prefix, when matched go to host/group
		// all other metrics are ignored
		dtest := []struct {
			metricName   string
			expectedDest string
			expectedName string
		}{
			{"host.foo", "host", "foo"},
			{"group.foo", "group", "foo"},
			{"foo", "ignore", "foo"},
		}
		viper.Set(config.KeyStatsdHostPrefix, "host.")
		viper.Set(config.KeyStatsdGroupPrefix, "group.")
		cfg := initSettings()
		for _, d := range dtest {
			t.Logf("%s -> %s", d.metricName, d.expectedDest)
			dest, metric := getMetricDestination(cfg, d.metricName)
			if dest != d.expectedDest {
				t.Fatalf("dest expected '%s' got '%s'", d.expectedDest, dest)
			}
			if metric != d.expectedName {
				t.Fatalf("name expected '%s' got '%s'", d.expectedName, metric)
			}
		}
		viper.Reset()
	}

	t.Log("Default to host")
	{
		// group metrics have a prefix, when matched go to group
		// all other metrics go to host
		dtest := []struct {
			metricName   string
			expectedDest string
			expectedName string
		}{
			{"host.foo", "host", "host.foo"},
			{"group.foo", "group", "foo"},
			{"foo", "host", "foo"},
		}
		viper.Set(config.KeyStatsdGroupPrefix, "group.")
		cfg := initSettings()
		for _, d := range dtest {
			t.Logf("%s -> %s", d.metricName, d.expectedDest)
			dest, metric := getMetricDestination(cfg, d.metricName)
			if dest != d.expectedDest {
				t.Fatalf("dest expected '%s' got '%s'", d.expectedDest, dest)
			}
			if metric != d.expectedName {
				t.Fatalf("name expected '%s' got '%s'", d.expectedName, metric)
			}
		}
		viper.Reset()
	}

	t.Log("Default to group")
	{
		// host metrics have a prefix, when matched go to host
		// all other metrics go to group
		dtest := []struct {
			metricName   string
			expectedDest string
			expectedName string
		}{
			{"host.foo", "host", "foo"},
			{"group.foo", "group", "group.foo"},
			{"foo", "group", "foo"},
		}
		viper.Set(config.KeyStatsdHostPrefix, "host.")
		cfg := initSettings()
		for _, d := range dtest {
			t.Logf("%s -> %s", d.metricName, d.expectedDest)
			dest, metric := getMetricDestination(cfg, d.metricName)
			if dest != d.expectedDest {
				t.Fatalf("dest expected '%s' got '%s'", d.expectedDest, dest)
			}
			if metric != d.expectedName {
				t.Fatalf("name expected '%s' got '%s'", d.expectedName, metric)
			}
		}
		viper.Reset()
	}
}

func TestParseMetric(t *testing.T) {
	t.Log("Testing parseMetric")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	cfg := initSettings()

	t.Log("Blank")
	{
		err := parseMetric(cfg, "")
		if err != nil {
			t.Fatalf("expected nil, got %s", err)
		}
	}

	if err := initHostMetrics(); err != nil {
		t.Fatalf("initHostMetrics %s", err)
	}

	mtests := []struct {
		metric string
		expect error
	}{
		{"test:1|c", nil},
		{"test:1|g", nil},
		{"test:1|h", nil},
		{"test:1|ms", nil},
		{"test:foo|s", nil},
		{"test:bar|t", nil},
		{"invalid-format", errors.New("invalid metric format 'invalid-format', ignoring")},
		{":invalid-no-name|t", errors.New("invalid metric format ':invalid-no-name|t', ignoring")},
		{"invalid-no-value:|t", errors.New("invalid metric format 'invalid-no-value:|t', ignoring")},
		{"invalid-rate:1|c@t", errors.New("invalid metric format 'invalid-rate:1|c@t', ignoring")},
		{"test:1.2|c", errors.New(`invalid counter value: strconv.ParseUint: parsing "1.2": invalid syntax`)},
		{"test:0|c", nil},
		{"test:1|c@.1", nil},
		{"test:0|g", nil},
		{"test:1|g", nil},
		{"test:1|g@.1", nil},
		{"test:1.0|g", nil},
		{"test:-1.0|g", nil},
		{"test:-1|g", nil},
		{"test:1.0.0|g", errors.New(`invalid gauge value: strconv.ParseFloat: parsing "1.0.0": invalid syntax`)},
		{"test:-1-|g", errors.New(`invalid gauge value: strconv.ParseInt: parsing "-1-": invalid syntax`)},
		{"test:1a|g", errors.New(`invalid gauge value: strconv.ParseUint: parsing "1a": invalid syntax`)},
		{"test:1|h", nil},
		{"test:1|ms", nil},
		{"test:1.0|h", nil},
		{"test:1.0|ms", nil},
		{"test:-1.0|h", nil},
		{"test:-1.0|ms", nil},
		{"test:1|h@.1", nil},
		{"test:1|ms@.1", nil},
		{"test:1.0a|h", errors.New(`invalid histogram value: strconv.ParseFloat: parsing "1.0a": invalid syntax`)},
		{"test:1.0a|ms", errors.New(`invalid histogram value: strconv.ParseFloat: parsing "1.0a": invalid syntax`)},
		{"test:1|q", errors.New("invalid metric type (q)")},
	}

	for _, mt := range mtests {
		t.Logf("Testing '%s' expect %v", mt.metric, mt.expect)
		err := parseMetric(cfg, mt.metric)
		if mt.expect == nil {
			if err != nil {
				t.Fatalf("expected nil, got (%s)", err)
			}
		} else {
			if err == nil {
				t.Fatal("expected error")
			}
			if mt.expect.Error() != err.Error() {
				t.Fatalf("expected (%s) got (%s)", mt.expect, err)
			}
		}
	}
}