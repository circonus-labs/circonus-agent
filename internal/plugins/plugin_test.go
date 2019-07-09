// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package plugins

import (
	"context"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

func TestDrain(t *testing.T) {
	t.Log("Testing Drain")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	p := &plugin{
		ctx:        context.Background(),
		id:         "test",
		instanceID: "",
		name:       "test",
		command:    path.Join("testdata", "test.sh"),
	}

	t.Log("blank w/o prevMetrics")
	{

		data := p.drain()
		if data == nil {
			t.Fatal("expected data")
		}
	}

	t.Log("blank w/prevMetrics")
	{
		p.prevMetrics = &cgm.Metrics{}

		data := p.drain()
		if data == nil {
			t.Fatal("expected data")
		}
	}
}

func TestParsePluginOutput(t *testing.T) {
	t.Log("Testing parsePluginOutput")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	p := &plugin{
		ctx:        context.Background(),
		id:         "test",
		instanceID: "",
		name:       "test",
		command:    path.Join("testdata", "test.sh"),
	}

	t.Log("blank")
	{
		p.metrics = nil
		expectedErr := errors.Errorf("zero lines of output")
		err := p.parsePluginOutput([]string{})
		if err == nil {
			t.Fatalf("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
		if len(*p.metrics) != 0 {
			t.Fatalf("expected 0 metrics, have (%#v)", p.metrics)
		}
	}

	t.Log("invalid json metric")
	{
		expectedErr := errors.Errorf("parsing json: unexpected end of JSON input")
		err := p.parsePluginOutput([]string{"{"})
		if err == nil {
			t.Fatalf("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
		if len(*p.metrics) != 0 {
			t.Fatalf("expected 0 metrics, have (%#v)", p.metrics)
		}
	}

	t.Log("json metric")
	{
		err := p.parsePluginOutput([]string{`{"metric": {"_type": "I", "_value": 22.1}}`})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len(*p.metrics) == 0 {
			t.Fatalf("expected 1 metric, have (%#v)", p.metrics)
		}
	}

	var tabDelimTests = []struct {
		description     string
		output          []string
		expectedMetrics int
	}{
		{"implied null", []string{"metric\tL"}, 1},
		{"explicit null", []string{"metric\tL\t[[null]]"}, 1},
		{"int32", []string{"metric\ti\t1"}, 1},
		{"uint32", []string{"metric\tI\t1"}, 1},
		{"int64", []string{"metric\tl\t1"}, 1},
		{"uint64", []string{"metric\tL\t1"}, 1},
		{"double", []string{"metric\tn\t1.0"}, 1},
		{"string", []string{"metric\ts\tfoo"}, 1},
		{"auto", []string{"metric\tO\tfoo"}, 1},
		{"invalid", []string{"metric\tQ\tfoo"}, 0},
		{"invalid int32", []string{"metric\ti\tfoo"}, 0},
		{"invalid uint32", []string{"metric\tI\tfoo"}, 0},
		{"invalid int64", []string{"metric\tl\tfoo"}, 0},
		{"invalid uint64", []string{"metric\tL\tfoo"}, 0},
		{"invalid double", []string{"metric\tn\tfoo"}, 0},
		{"invalid delimiter", []string{"metric L 1"}, 0},
		{"invalid number of fields", []string{"metric\tL\t1\tfoo\tbar"}, 0},
		{"invalid metric type", []string{"metric\tfoo\t1"}, 0},
		{"invalid metric type", []string{"metric\t\t1"}, 0},
	}

	for _, tdt := range tabDelimTests {
		t.Logf("tab delimited - %s (%#v)", tdt.description, tdt.output)
		p.metrics = nil
		err := p.parsePluginOutput(tdt.output)
		if err != nil {
			t.Fatalf("expected NO error, got (%s) - test output: %#v", err, tdt.output)
		}
		if len(*p.metrics) != tdt.expectedMetrics {
			t.Fatalf("expected %d metric(s), have (%#v) - test output: %#v", tdt.expectedMetrics, p.metrics, tdt.output)
		}
	}
}

func TestExec(t *testing.T) {
	t.Log("Testing exec")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	p := &plugin{
		ctx:        context.Background(),
		id:         "test",
		instanceID: "",
		name:       "test",
		command:    path.Join("testdata", "test.sh"),
	}

	// tags agent v1+ will add by default
	baseTags := cgm.Tags{
		cgm.Tag{Category: "collector", Value: "test"},
		cgm.Tag{Category: "source", Value: "circonus-agent"},
	}

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("unable to get cwd (%s)", err)
	}
	testDir := path.Join(dir, "testdata")

	t.Log("already running")
	{
		p.running = true
		expectedErr := errors.Errorf("already running")
		err := p.exec()
		if err == nil {
			t.Fatalf("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
		p.running = false
	}

	t.Log("TTL not expired")
	{
		p.lastEnd = time.Now()
		p.runTTL = 5 * time.Minute
		expectedErr := errors.Errorf("TTL not expired")
		err := p.exec()
		if err == nil {
			t.Fatalf("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
		p.runTTL = time.Duration(0)
	}

	t.Log("not found")
	{
		p.command = path.Join("testdata", "invalid")
		err := p.exec()
		if err == nil {
			t.Fatalf("expected error")
		}
	}

	t.Log("not found (in $PATH)")
	{
		p.command = "invalid"
		err := p.exec()
		if err == nil {
			t.Fatalf("expected error")
		}
	}

	t.Log("error (exit)")
	{
		p.command = path.Join(testDir, "error.sh")
		err := p.exec()
		if err == nil {
			t.Fatalf("expected error")
		}
	}

	t.Log("args")
	{

		if runtime.GOOS == "windows" {
			p.command = path.Join(testDir, "argswin.bat")
		} else {
			p.command = path.Join(testDir, "args.sh")
		}
		p.instanceArgs = []string{"foo", "bar"}
		err := p.exec()
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if len(*p.metrics) == 0 {
			t.Fatal("expected metrics")
		}
		var tagList cgm.Tags
		tagList = append(tagList, baseTags...)
		metricName := tags.MetricNameWithStreamTags(strings.Join(p.instanceArgs, "`"), tagList)
		_, ok := (*p.metrics)[metricName]
		if !ok {
			t.Fatalf("expected '%s' metric, got (%v)", metricName, *p.metrics)
		}
	}
}
