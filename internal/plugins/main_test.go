// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package plugins

import (
	"bytes"
	"os"
	"path"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestScan(t *testing.T) {
	t.Log("Testing Scan")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("No plugin directory")
	{
		viper.Set(config.KeyPluginDir, "")

		expectErr := errors.Errorf("plugin directory scan: invalid plugin directory (none)")
		p := New()
		err := p.Scan()
		if err == nil {
			t.Fatal("expected error")
		}
		if expectErr.Error() != err.Error() {
			t.Fatalf("expected (%s) got (%s)", expectErr, err)
		}
	}

	t.Log("Valid plugin directory")
	{
		viper.Set(config.KeyPluginDir, "testdata/")

		p := New()
		err := p.Scan()
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

}

func TestRun(t *testing.T) {
	t.Log("Testing Run")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("unable to get cwd (%s)", err)
	}
	viper.Set(config.KeyPluginDir, path.Join(dir, "testdata"))
	p := New()

	if err := p.Scan(); err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}

	// let initialization, first run, etc. complete
	// otherwise, there can be a race condition if Run is called to quickly resulting in failed tests
	time.Sleep(2 * time.Second)

	t.Log("Invalid (already running)")
	{
		p.running = true
		expectedErr := errors.New("plugin run already in progress")
		err := p.Run("invalid")
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
		p.running = false
	}

	t.Log("Invalid (unknown plugin)")
	{
		expectedErr := errors.New("invalid plugin (invalid)")
		err := p.Run("invalid")
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
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

	p := New()
	if err := p.Scan(); err != nil {
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
		data := p.Flush("test")
		if len(*data) == 0 {
			t.Fatalf("expected metrics got (%#v)", data)
		}
		metrics := (*data)["test"].(*Metrics)
		if len(*metrics) == 0 {
			t.Fatalf("expected metrics, got (%#v)", metrics)
		}
		metric, ok := (*metrics)["metric"]
		if !ok {
			t.Fatalf("expected metric named 'metric'")
		}
		if metric.Type != "n" {
			t.Fatalf("expected type 'n', got %#v", metric)
		}
		if metric.Value.(float64) != 22.1 {
			t.Fatalf("expected value '22.1', got %#v", metric)
		}
	}
}

func TestIsValid(t *testing.T) {
	t.Log("Testing IsValid")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	viper.Set(config.KeyPluginDir, "testdata/")
	p := New()

	err := p.Scan()
	if err != nil {
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

	viper.Set(config.KeyPluginDir, "testdata/")

	p := New()
	err := p.Scan()
	if err != nil {
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

	viper.Set(config.KeyPluginDir, "testdata/")

	p := New()
	err := p.Scan()
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}

	t.Log("Valid")
	{
		data, err := p.Inventory()
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}

		expect := []byte(`"test":{"ID":"test","InstanceID":"","Name":"test","InstanceArgs":null,"Command":"testdata/test.sh","Generation":1`)
		if !bytes.Contains(data, expect) {
			t.Fatalf("expected (%s) got (%s)", string(expect), string(data))
		}
	}
}
