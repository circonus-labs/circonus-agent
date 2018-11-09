// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package plugins

import (
	"bytes"
	"context"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestNew(t *testing.T) {
	t.Log("Testing New")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	tests := []struct {
		id         string
		defaultDir string
		dir        string
		list       []string
		shouldFail bool
		errMsg     string
	}{
		{"invalid - both dir/list specified", "testdata", "testdata", []string{path.Join("testdata", "test.sh")}, true, "invalid configuration cannot specifiy plugin-dir AND plugin-list"},
		{"invalid - not a dir", "testdata", path.Join("testdata", "test.sh"), []string{}, true, "Invalid plugin directory"},
		{"valid - no dir/list, default to dir", "testdata", "", []string{}, false, ""},
		{"valid - dir", "", "testdata", []string{}, false, ""},
		{"valid - list", "", "", []string{path.Join("testdata", "test.sh")}, false, ""},
	}

	for _, test := range tests {
		tst := test
		t.Run(tst.id, func(t *testing.T) {
			viper.Set(config.KeyPluginList, tst.list)
			viper.Set(config.KeyPluginDir, tst.dir)
			_, err := New(context.Background(), tst.defaultDir)
			if tst.shouldFail {
				if err == nil {
					t.Fatal("expected error")
				} else if !strings.HasPrefix(err.Error(), tst.errMsg) {
					t.Fatalf("unexpected error (%s)", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error (%s)", err)
				}
			}
		})
	}
}

func TestRun(t *testing.T) {
	t.Log("Testing Run")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("unable to get cwd (%s)", err)
	}
	viper.Reset()
	viper.Set(config.KeyPluginDir, path.Join(dir, "testdata"))
	p, nerr := New(context.Background(), "")
	if nerr != nil {
		t.Fatalf("new err %s", nerr)
	}

	b, err := builtins.New()
	if err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}

	if err := p.Scan(b); err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}

	// let initialization, first run, etc. complete
	// otherwise, there can be a race condition if Run is called to quickly resulting in failed tests
	time.Sleep(2 * time.Second)

	t.Log("Invalid (already running)")
	{
		p.running = true
		err := p.Run("invalid")
		if err == nil {
			t.Fatal("expected error")
		}
		p.running = false
	}

	t.Log("Invalid (unknown plugin)")
	{
		err := p.Run("invalid")
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("Valid (all)")
	{
		err := p.Run("")
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("Valid (one)")
	{
		err := p.Run("test")
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}
}

func TestFlush(t *testing.T) {
	t.Log("Testing Flush")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	viper.Reset()
	p, nerr := New(context.Background(), "testdata/")
	if nerr != nil {
		t.Fatalf("new err %s", nerr)
	}

	b, err := builtins.New()
	if err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}

	if err := p.Scan(b); err != nil {
		t.Fatalf("expected no error, got %s", err)
	}
	time.Sleep(2 * time.Second)
	if err := p.Run("test"); err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}

	time.Sleep(2 * time.Second)

	t.Log("Invalid")
	{
		data := p.Flush("invalid")
		if data == nil {
			t.Fatal("expected data")
		}
		if len(*data) != 0 {
			t.Fatalf("expected no metrics, got (%#v)", data)
		}
	}

	t.Log("Valid")
	{
		id := "test"
		if runtime.GOOS == "windows" {
			id = "testwin"
		}
		data := p.Flush(id)
		if data == nil {
			t.Fatal("expected not nil")
		}
		if len(*data) == 0 {
			t.Fatalf("expected metrics got none (%#v)", data)
		}

		name := "test"
		if runtime.GOOS == "windows" {
			name = "testwin"
		}
		name += metricDelimiter + "metric"

		mv, ok := (*data)[name]
		if !ok {
			t.Fatalf("expected metric named (%s) got (%#v)", name, *data)
		}
		if mv.Type != "n" {
			t.Fatalf("expected type 'n', got %#v", mv)
		}
		if mv.Value.(float64) != 22.1 {
			t.Fatalf("expected value 22.1 got %#v", mv)
		}
	}
}

func TestIsValid(t *testing.T) {
	t.Log("Testing IsValid")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	viper.Reset()
	viper.Set(config.KeyPluginDir, "testdata")
	p, nerr := New(context.Background(), "")
	if nerr != nil {
		t.Fatalf("new err %s", nerr)
	}

	b, err := builtins.New()
	if err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}

	if err := p.Scan(b); err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}

	t.Log("Invalid []")
	{
		ok := p.IsValid("")
		if ok {
			t.Fatal("expected false")
		}
	}

	t.Log("Invalid [invalid]")
	{
		ok := p.IsValid("invalid")
		if ok {
			t.Fatal("expected false")
		}
	}

	t.Log("Valid")
	{
		ok := p.IsValid("test")
		if !ok {
			t.Fatal("expected true")
		}
	}

}

func TestIsInternal(t *testing.T) {
	t.Log("Testing IsInternal")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	viper.Reset()
	viper.Set(config.KeyPluginDir, "testdata")

	p, nerr := New(context.Background(), "")
	if nerr != nil {
		t.Fatalf("new err %s", nerr)
	}

	b, err := builtins.New()
	if err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}

	if err := p.Scan(b); err != nil {
		t.Fatalf("expected no error, got %s", err)
	}

	t.Log("Internal - statsd")
	{
		internal := p.IsInternal("statsd")
		if !internal {
			t.Fatal("expected true")
		}
	}

	t.Log("Internal - write")
	{
		internal := p.IsInternal("write")
		if !internal {
			t.Fatal("expected true")
		}
	}

	t.Log("Not internal - blank")
	{
		internal := p.IsInternal("")
		if internal {
			t.Fatal("expected false")
		}
	}

	t.Log("Not internal - foo")
	{
		internal := p.IsInternal("foo")
		if internal {
			t.Fatal("expected false")
		}
	}

}

func TestInventory(t *testing.T) {
	t.Log("Testing Inventory")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	viper.Reset()
	viper.Set(config.KeyPluginDir, "testdata")

	p, nerr := New(context.Background(), "")
	if nerr != nil {
		t.Fatalf("new err %s", nerr)
	}

	b, err := builtins.New()
	if err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}

	p.pluginDir = "testdata" // set it back to relative so absolute path does not make test fail below

	if err := p.Scan(b); err != nil {
		t.Fatalf("expected no error, got %s", err)
	}

	t.Log("Valid")
	{
		data := p.Inventory()
		if data == nil {
			t.Fatalf("expected not nil")
		}

		expect := []byte(`{"id":"test","name":"test","instance":"","command":"testdata/test.sh"`)
		if !bytes.Contains(data, expect) {
			t.Fatalf("expected (%s) got (%s)", string(expect), string(data))
		}
	}
}
