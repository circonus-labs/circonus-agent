// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"net/url"
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/go-apiclient"
	"github.com/gojuno/minimock/v3"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestBrokerTLSConfig(t *testing.T) {
	t.Log("Testing brokerTLSConfig")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	mc := minimock.NewController(t)
	client := genMockClient(mc)

	rurl, err := url.Parse("http://127.0.0.1:1234/")
	if err != nil {
		t.Fatalf("parsing test url (%s)", err)
	}

	t.Log("cid (empty)")
	{
		c := Check{}
		_, _, err := c.brokerTLSConfig("", rurl)

		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "invalid broker cid (empty)" {
			t.Fatalf("unexpected error (%s)", err)
		}
	}

	t.Log("cid (invalid)")
	{
		c := Check{}
		_, _, err := c.brokerTLSConfig("foo", rurl)

		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "invalid broker cid (foo)" {
			t.Fatalf("unexpected error (%s)", err)
		}
	}

	t.Log("api error")
	{
		c := Check{client: client}
		_, _, err := c.brokerTLSConfig("/broker/000", rurl)
		viper.Reset()

		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "unable to retrieve broker (/broker/000): forced mock api call error" {
			t.Fatalf("unexpected error (%s)", err)
		}
	}

	t.Log("host not matched")
	{
		badrurl, uerr := url.Parse("http://127.0.0.2:1234/")
		if uerr != nil {
			t.Fatalf("parsing test url (%s)", uerr)
		}

		c := Check{client: client}
		_, _, err := c.brokerTLSConfig("/broker/1234", badrurl)
		viper.Reset()

		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "unable to match reverse URL host (127.0.0.2) to broker" {
			t.Fatalf("unexpected error (%s)", err)
		}
	}

	t.Log("bad file cert")
	{
		c := Check{client: client}
		viper.Set(config.KeyReverseBrokerCAFile, "testdata/missingca.crt")
		_, _, err := c.brokerTLSConfig("/broker/1234", rurl)
		viper.Reset()

		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "reading specified broker-ca-file (testdata/missingca.crt): open testdata/missingca.crt: no such file or directory" {
			t.Fatalf("unexpected error (%s)", err)
		}
	}

	t.Log("valid w/file cert")
	{
		c := Check{client: client}
		viper.Set(config.KeyReverseBrokerCAFile, "testdata/ca.crt")
		_, _, err := c.brokerTLSConfig("/broker/1234", rurl)
		viper.Reset()

		if err != nil {
			t.Fatalf("expected NO error got (%s)", err)
		}
	}

	t.Log("valid w/api cert (full cid)")
	{
		c := Check{client: client}
		_, _, err := c.brokerTLSConfig("/broker/1234", rurl)
		viper.Reset()

		if err != nil {
			t.Fatalf("expected NO error got (%s)", err)
		}
	}

	t.Log("valid w/api cert (# cid)")
	{
		c := Check{client: client}
		_, _, err := c.brokerTLSConfig("1234", rurl)
		viper.Reset()

		if err != nil {
			t.Fatalf("expected NO error got (%s)", err)
		}
	}
}

func TestBrokerSupportsCheckType(t *testing.T) {
	t.Log("Testing brokerSupportsCheckType")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	details := apiclient.BrokerDetail{Modules: []string{"json", "httptrap"}}

	t.Log("unsupported (foo)")
	{
		checkType := "foo"
		if brokerSupportsCheckType(checkType, &details) {
			t.Fatal("expected unsupported")
		}
	}

	t.Log("supported (json:nad)")
	{
		checkType := "json:nad"
		if !brokerSupportsCheckType(checkType, &details) {
			t.Fatal("expected supported")
		}
	}

	t.Log("supported (httptrap)")
	{
		checkType := "httptrap"
		if !brokerSupportsCheckType(checkType, &details) {
			t.Fatal("expected supported")
		}
	}
}
