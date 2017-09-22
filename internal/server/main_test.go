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
		s := New(nil)
		if s.svrHTTP != nil {
			t.Fatal("expected nil")
		}
	}

	t.Log("With config")
	{
		viper.Set(config.KeyListen, ":2609")
		s := New(nil)
		if s.svrHTTP == nil {
			t.Fatal("expected NOT nil")
		}
	}
}

func TestServerHTTPS(t *testing.T) {
	t.Log("Testing serverHTTPS")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("No config")
	{
		s := New(nil)
		if s.svrHTTPS != nil {
			t.Fatal("expected nil")
		}
	}

	t.Log("With config")
	{
		viper.Set(config.KeySSLListen, ":2610")
		s := New(nil)
		viper.Reset()
		if s.svrHTTPS == nil {
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
		s := New(nil)
		time.AfterFunc(2*time.Second, func() {
			s.svrHTTP.Close()
		})
		if err := s.Start(); err != nil {
			t.Fatalf("expected NO error, got (%v)", err)
		}
		viper.Reset()
	}

	t.Log("HTTPS (no cert/key config)")
	{
		viper.Set(config.KeySSLListen, ":65222")
		s := New(nil)
		expectedErr := errors.New("SSL server: open : no such file or directory")
		err := s.Start()
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%v)", expectedErr, err)
		}
		s.svrHTTPS.Close()
		viper.Reset()
	}

	t.Log("HTTPS (no cert)")
	{
		viper.Set(config.KeySSLListen, ":65223")
		viper.Set(config.KeySSLCertFile, "testdata/missing.crt")
		s := New(nil)
		expectedErr := errors.New("SSL server: open testdata/missing.crt: no such file or directory")
		err := s.Start()
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%v)", expectedErr, err)
		}
		s.svrHTTPS.Close()
		viper.Reset()
	}

	t.Log("HTTPS (no key)")
	{
		viper.Set(config.KeySSLListen, ":65224")
		viper.Set(config.KeySSLCertFile, "testdata/cert.crt")
		viper.Set(config.KeySSLKeyFile, "testdata/missing.key")
		s := New(nil)
		expectedErr := errors.New("SSL server: open testdata/missing.key: no such file or directory")
		err := s.Start()
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%v)", expectedErr, err)
		}
		s.svrHTTPS.Close()
		viper.Reset()
	}

	t.Log("HTTPS cert/key fail")
	{
		viper.Set(config.KeySSLListen, ":65225")
		viper.Set(config.KeySSLCertFile, "testdata/cert.crt")
		viper.Set(config.KeySSLKeyFile, "testdata/key.key")
		s := New(nil)
		expectedErr := errors.New("SSL server: tls: failed to find any PEM data in certificate input")
		err := s.Start()
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%v)", expectedErr, err)
		}
		s.svrHTTPS.Close()
		viper.Reset()
	}
}
