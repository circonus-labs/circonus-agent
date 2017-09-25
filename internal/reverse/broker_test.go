// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestGetTLSConfig(t *testing.T) {
	t.Log("Testing getTLSConfig")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	rurl, err := url.Parse("http://127.0.0.1:1234/")
	if err != nil {
		t.Fatalf("parsing test url (%s)", err)
	}

	badrurl, err := url.Parse("http://127.0.0.2:1234/")
	if err != nil {
		t.Fatalf("parsing test url (%s)", err)
	}

	t.Log("No broker cid")
	{
		viper.Set(config.KeyReverse, false)
		c, _ := New(context.Background())

		_, err := c.getTLSConfig("", rurl)

		expectedErr := errors.New("No broker CID supplied")
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("Invalid broker cid")
	{
		viper.Set(config.KeyReverse, false)
		c, _ := New(context.Background())

		_, err := c.getTLSConfig("1234", rurl)

		expectedErr := errors.New("Invalid broker CID (1234)")
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("No API token")
	{
		viper.Set(config.KeyReverse, false)
		c, _ := New(context.Background())

		_, err := c.getTLSConfig("/broker/1234", rurl)

		expectedErr := errors.New("Initializing cgm API: API Token is required")
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("broker not found")
	{
		viper.Set(config.KeyAPIURL, apiSim.URL)
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "foo")
		viper.Set(config.KeyReverse, false)
		c, _ := New(context.Background())
		_, err := c.getTLSConfig("/broker/404", rurl)
		viper.Reset()

		expectedErr := errors.New("Fetching broker (/broker/404) from API: [ERROR] API response code 404: not found")
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("host not matched")
	{
		viper.Set(config.KeyAPIURL, apiSim.URL)
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "foo")
		viper.Set(config.KeyReverse, false)
		c, _ := New(context.Background())
		_, err := c.getTLSConfig("/broker/1234", badrurl)
		viper.Reset()

		expectedErr := errors.New("Unable to match reverse URL host (127.0.0.2) to broker")
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("bad file cert")
	{
		viper.Set(config.KeyAPIURL, apiSim.URL)
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "foo")
		viper.Set(config.KeyReverse, false)
		c, _ := New(context.Background())
		viper.Set(config.KeyReverseBrokerCAFile, "testdata/missingca.crt")
		_, err := c.getTLSConfig("/broker/1234", rurl)
		viper.Reset()

		expectedErr := errors.New("Reading specified broker-ca-file (testdata/missingca.crt): open testdata/missingca.crt: no such file or directory")
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
	}

	t.Log("valid w/file cert")
	{
		viper.Set(config.KeyAPIURL, apiSim.URL)
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "foo")
		viper.Set(config.KeyReverseBrokerCAFile, "testdata/ca.crt")
		viper.Set(config.KeyReverse, false)
		c, _ := New(context.Background())
		_, err := c.getTLSConfig("/broker/1234", rurl)
		viper.Reset()

		if err != nil {
			t.Fatalf("expected NO error got (%s)", err)
		}
	}

	t.Log("valid w/api cert")
	{
		viper.Set(config.KeyAPIURL, apiSim.URL)
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "foo")
		viper.Set(config.KeyReverse, false)
		c, _ := New(context.Background())
		_, err := c.getTLSConfig("/broker/1234", rurl)
		viper.Reset()

		if err != nil {
			t.Fatalf("expected NO error got (%s)", err)
		}
	}
}
