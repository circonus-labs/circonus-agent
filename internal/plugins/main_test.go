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
	"strings"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins"
	"github.com/circonus-labs/circonus-agent/internal/config"
	cgm "github.com/circonus-labs/circonus-gometrics"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestNew(t *testing.T) {
	t.Log("Testing New")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("invalid - empty string")
	{
		viper.Set(config.KeyPluginDir, "")

		expectErr := errors.Errorf("Invalid plugin directory (none)")
		_, err := New(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
		if expectErr.Error() != err.Error() {
			t.Fatalf("expected (%s) got (%s)", expectErr, err)
		}
	}

	t.Log("invalid - not a directory")
	{
		viper.Set(config.KeyPluginDir, "testdata/test.sh")

		expect := "Invalid plugin directory (/"
		_, err := New(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.HasPrefix(err.Error(), expect) {
			t.Fatalf("expected (^%s) got (%s)", expect, err)
		}
	}

	t.Log("invalid - no access")
	{
		viper.Set(config.KeyPluginDir, "testdata/noaccess")

		expect := "Invalid plugin directory: open /"
		_, err := New(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.HasPrefix(err.Error(), expect) {
			t.Fatalf("expected (^%s) got (%s)", expect, err)
		}
		expect = "permission denied"
		if !strings.Contains(err.Error(), expect) {
			t.Fatalf("expected (*%s*) got (%s)", expect, err)
		}
	}

	t.Log("valid plugin directory")
	{
		viper.Set(config.KeyPluginDir, "testdata/")

		_, err := New(context.Background())
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
	p, nerr := New(context.Background())
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

	p, nerr := New(context.Background())
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
		data := p.Flush("test")
		if len(*data) == 0 {
			t.Fatalf("expected metrics got (%#v)", data)
		}
		metrics := (*data)["test"].(*cgm.Metrics)
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
	p, nerr := New(context.Background())
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

	viper.Set(config.KeyPluginDir, "testdata/")

	p, nerr := New(context.Background())
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

	viper.Set(config.KeyPluginDir, "testdata/")

	p, nerr := New(context.Background())
	if nerr != nil {
		t.Fatalf("new err %s", nerr)
	}

	b, err := builtins.New()
	if err != nil {
		t.Fatalf("expected NO error, got (%s)", err)
	}

	p.pluginDir = "testdata/" // set it back to relative so absolute path does not make test fail below

	if err := p.Scan(b); err != nil {
		t.Fatalf("expected no error, got %s", err)
	}

	t.Log("Valid")
	{
		data := p.Inventory()
		if data == nil {
			t.Fatalf("expected not nil")
		}

		expect := []byte(`"test":{"name":"test","instance":"","command":"testdata/test.sh","args":null`)
		if !bytes.Contains(data, expect) {
			t.Fatalf("expected (%s) got (%s)", string(expect), string(data))
		}
	}
}
