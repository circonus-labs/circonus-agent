// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"testing"

	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/go-apiclient"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestGetFullCheckMetrics(t *testing.T) {
	t.Log("Testing getFullCheckMetrics")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	client := genMockClient()
	c := Check{bundle: &apiclient.CheckBundle{CID: ""}, client: client}

	t.Log("api error")
	{
		c.bundle.CID = "/check_bundle/000"
		_, err := c.getFullCheckMetrics()
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "fetching check bundle metrics: forced mock api call error" {
			t.Fatalf("unexpected error (%s)", err)
		}
	}

	t.Log("parse error")
	{
		c.bundle.CID = "/check_bundle/0001"
		_, err := c.getFullCheckMetrics()
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "parsing check bundle metrics: unexpected end of JSON input" {
			t.Fatalf("unexpected error (%s)", err)
		}
	}

	t.Log("valid")
	{
		c.bundle.CID = "/check_bundle/1234"
		m, err := c.getFullCheckMetrics()
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}

		if len(*m) == 0 {
			t.Fatal("expected > 0 metrics")
		}
	}
}

func TestUpdateCheckBundleMetrics(t *testing.T) {
	t.Log("Testing updateCheckBundleMetrics")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	client := genMockClient()
	c := Check{bundle: &apiclient.CheckBundle{CID: ""}, client: client}

	t.Log("nil metrics")
	{
		err := c.updateCheckBundleMetrics(nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "nil metrics passed to update" {
			t.Fatalf("unexpected error (%s)", err)
		}
	}

	t.Log("short circuit (0 metrics)")
	{
		metrics := map[string]apiclient.CheckBundleMetric{}
		err := c.updateCheckBundleMetrics(&metrics)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
	}

	t.Log("api error")
	{
		c.bundle.CID = "/check_bundle/000"
		metrics := map[string]apiclient.CheckBundleMetric{
			"foo": apiclient.CheckBundleMetric{Name: "foo", Type: "n", Status: "active"},
		}
		err := c.updateCheckBundleMetrics(&metrics)
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "unable to fetch up-to-date copy of check: forced mock api call error" {
			t.Fatalf("unexpected error (%s)", err)
		}
	}

	t.Log("update error")
	{
		c.bundle.CID = "/check_bundle/0002"
		metrics := map[string]apiclient.CheckBundleMetric{
			"foo": apiclient.CheckBundleMetric{Name: "foo", Type: "n", Status: "active"},
		}
		err := c.updateCheckBundleMetrics(&metrics)
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "unable to update check bundle with new metrics: api update check bundle error" {
			t.Fatalf("unexpected error (%s)", err)
		}
	}

	t.Log("new metric")
	{
		c.bundle.CID = "/check_bundle/1234"
		metrics := map[string]apiclient.CheckBundleMetric{
			"foo": apiclient.CheckBundleMetric{Name: "foo", Type: "n", Status: "active"},
		}
		err := c.updateCheckBundleMetrics(&metrics)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		_, ok := (*c.metricStates)["foo"]
		if !ok {
			t.Fatalf("expected metric named 'foo', metric states (%#v)", c.metricStates)
		}
	}
}

func TestConfigMetric(t *testing.T) {
	t.Log("Testing configMetric")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	c := Check{logger: log.Logger}
	cases := []struct {
		desc string
		mn   string
		mv   cgm.Metric
		mt   string
	}{
		{"int32", "foo", cgm.Metric{Type: "i", Value: int32(1)}, "numeric"},
		{"uint32", "foo", cgm.Metric{Type: "I", Value: uint32(1)}, "numeric"},
		{"int64", "foo", cgm.Metric{Type: "l", Value: int64(1)}, "numeric"},
		{"uint64", "foo", cgm.Metric{Type: "L", Value: uint64(1)}, "numeric"},
		{"double/float32", "foo", cgm.Metric{Type: "n", Value: float32(1.0)}, "numeric"},
		{"double/float64", "foo", cgm.Metric{Type: "n", Value: float64(1.0)}, "numeric"},
		{"string", "foo", cgm.Metric{Type: "s", Value: "bar"}, "text"},
		{"numeric", "foo", cgm.Metric{Type: "n", Value: []float64{1.0, 2.0, 3.0}}, "histogram"},
		{"numeric", "foo", cgm.Metric{Type: "n", Value: [...]string{"H[1.0]=1", "H[2.0]=1", "H[3.0]=1"}}, "histogram"},
	}

	for _, tc := range cases {
		t.Logf("\t%s", tc.desc)
		m := c.configMetric(tc.mn, tc.mv)
		if m.Name != tc.mn {
			t.Fatalf("expected '%s' name in %#v", tc.mn, m)
		}
		if m.Type != tc.mt {
			t.Fatalf("expected '%s' type in %#v", tc.mt, m)
		}
	}
}
