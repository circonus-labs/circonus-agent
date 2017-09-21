// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"errors"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestServerHTTP(t *testing.T) {
	t.Log("Testing serverHTTP")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("No config")
	{
		server := serverHTTP()

		if server != nil {
			t.Fatal("expected nil")
		}
	}

	t.Log("With config")
	{
		viper.Set(config.KeyListen, ":2609")
		server := serverHTTP()
		if server == nil {
			t.Fatal("expected NOT nil")
		}
	}
}

func TestServerHTTPS(t *testing.T) {
	t.Log("Testing serverHTTPS")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("No config")
	{
		server := serverHTTPS()

		if server != nil {
			t.Fatal("expected nil")
		}
	}

	t.Log("With config")
	{
		viper.Set(config.KeySSLListen, ":2610")
		server := serverHTTPS()
		if server == nil {
			t.Fatal("expected NOT nil")
		}
	}
}

func TestRunServers(t *testing.T) {
	t.Log("Testing runServers")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("HTTP")
	{
		viper.Set(config.KeyListen, ":65111")
		httpServer := serverHTTP()

		time.AfterFunc(2*time.Second, func() {
			httpServer.Close()
		})

		if err := runServers(httpServer, nil); err != nil {
			t.Fatalf("expected NO error, got (%v)", err)
		}
		viper.Reset()
	}

	t.Log("HTTPS (no cert/key config)")
	{
		viper.Set(config.KeySSLListen, ":65222")
		httpsServer := serverHTTPS()
		expectedErr := errors.New("SSL server: open : no such file or directory")
		err := runServers(nil, httpsServer)
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%v)", expectedErr, err)
		}
		httpsServer.Close()
		viper.Reset()
	}

	t.Log("HTTPS (no cert)")
	{
		viper.Set(config.KeySSLListen, ":65223")
		viper.Set(config.KeySSLCertFile, "testdata/missing.crt")
		httpsServer := serverHTTPS()
		expectedErr := errors.New("SSL server: open testdata/missing.crt: no such file or directory")
		err := runServers(nil, httpsServer)
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%v)", expectedErr, err)
		}
		httpsServer.Close()
		viper.Reset()
	}

	t.Log("HTTPS (no key)")
	{
		viper.Set(config.KeySSLListen, ":65224")
		viper.Set(config.KeySSLCertFile, "testdata/cert.crt")
		viper.Set(config.KeySSLKeyFile, "testdata/missing.key")
		httpsServer := serverHTTPS()
		expectedErr := errors.New("SSL server: open testdata/missing.key: no such file or directory")
		err := runServers(nil, httpsServer)
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%v)", expectedErr, err)
		}
		httpsServer.Close()
		viper.Reset()
	}

	t.Log("HTTPS cert/key fail")
	{
		viper.Set(config.KeySSLListen, ":65225")
		viper.Set(config.KeySSLCertFile, "testdata/cert.crt")
		viper.Set(config.KeySSLKeyFile, "testdata/key.key")
		httpsServer := serverHTTPS()
		expectedErr := errors.New("SSL server: tls: failed to find any PEM data in certificate input")
		err := runServers(nil, httpsServer)
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%v)", expectedErr, err)
		}
		httpsServer.Close()
		viper.Reset()
	}
}
