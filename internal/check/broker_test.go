// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"net/url"
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/gojuno/minimock/v3"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestBrokerTLSConfig(t *testing.T) {
	t.Log("Testing brokerTLSConfig")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	mc := minimock.NewController(t)
	client := genMockClient(mc)

	rurl, err := url.Parse("http://192.168.1.1:1234/")
	if err != nil {
		t.Fatalf("parsing test url (%s)", err)
	}

	t.Log("no broker (empty)")
	{
		c := Check{}
		_, _, err := c.brokerTLSConfig(rurl)

		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "broker not initialized" {
			t.Fatalf("unexpected error (%s)", err)
		}
	}

	t.Log("host not matched")
	{
		badrurl, uerr := url.Parse("http://1.2.3.4:1234/")
		if uerr != nil {
			t.Fatalf("parsing test url (%s)", uerr)
		}

		c := Check{client: client, broker: &testBroker}
		_, _, err := c.brokerTLSConfig(badrurl)
		viper.Reset()

		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "unable to match reverse URL host (1.2.3.4) to broker" {
			t.Fatalf("unexpected error (%s)", err)
		}
	}

	t.Log("bad file cert")
	{
		c := Check{client: client, broker: &testBroker}
		viper.Set(config.KeyReverseBrokerCAFile, "testdata/missingca.crt")
		_, _, err := c.brokerTLSConfig(rurl)
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
		c := Check{client: client, broker: &testBroker}
		viper.Set(config.KeyReverseBrokerCAFile, "testdata/ca.crt")
		_, _, err := c.brokerTLSConfig(rurl)
		viper.Reset()

		if err != nil {
			t.Fatalf("expected NO error got (%s)", err)
		}
	}

	t.Log("valid w/api cert")
	{
		c := Check{client: client, broker: &testBroker}
		_, _, err := c.brokerTLSConfig(rurl)
		viper.Reset()

		if err != nil {
			t.Fatalf("expected NO error got (%s)", err)
		}
	}
}
