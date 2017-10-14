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
	viper.Reset()
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("No config")
	{
		s, _ := New(nil, nil)
		if s.svrHTTP != nil {
			t.Fatal("expected nil")
		}
	}

	t.Log("With config")
	{
		viper.Set(config.KeyListen, ":2609")
		s, _ := New(nil, nil)
		if s.svrHTTP == nil {
			t.Fatal("expected NOT nil")
		}
		viper.Reset()
	}
}

func TestServerHTTPS(t *testing.T) {
	t.Log("Testing serverHTTPS")
	viper.Reset()
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("No config")
	{
		s, _ := New(nil, nil)
		if s.svrHTTPS != nil {
			t.Fatal("expected nil")
		}
	}

	t.Log("With config")
	{
		viper.Set(config.KeySSLListen, ":2610")
		s, _ := New(nil, nil)
		viper.Reset()
		if s.svrHTTPS == nil {
			t.Fatal("expected NOT nil")
		}
		viper.Reset()
	}
}

func TestServerSocket(t *testing.T) {
	t.Log("Testing serverSocket")
	viper.Reset()
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("no config")
	{
		s, _ := New(nil, nil)
		if s.svrSocket != nil {
			t.Fatal("expected nil")
		}
	}

	t.Log("w/bad config")
	{
		viper.Set(config.KeyListenSocketPath, "nodir/test.sock")
		s, _ := New(nil, nil)
		if s.svrSocket == nil {
			t.Fatal("expected NOT nil")
		}
		expect := errors.New("listen unix nodir/test.sock: bind: no such file or directory")
		err := s.startSocket()
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expect.Error() {
			t.Fatalf("expected (%s) got (%v)", expect, err)
		}
		s.svrSocket.Close()
		viper.Reset()
	}

	t.Log("w/valid config")
	{
		viper.Set(config.KeyListenSocketPath, "testdata/test.sock")
		s, _ := New(nil, nil)
		viper.Reset()
		if s.svrSocket == nil {
			t.Fatal("expected NOT nil")
		}
		viper.Reset()
	}
}

func TestStart(t *testing.T) {
	t.Log("Testing Start")
	viper.Reset()
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("no servers")
	{
		s, _ := New(nil, nil)
		err := s.Start()
		if err == nil {
			t.Fatal("expected error")
		}
	}

	t.Log("HTTP")
	{
		viper.Set(config.KeyListen, ":65111")
		s, _ := New(nil, nil)
		time.AfterFunc(1*time.Second, func() {
			s.Stop()
		})
		if err := s.Start(); err != nil {
			t.Fatalf("expected NO error, got (%v)", err)
		}
		viper.Reset()
	}

	t.Log("HTTPS (no cert/key config)")
	{
		viper.Set(config.KeySSLListen, ":65222")
		s, _ := New(nil, nil)
		expectedErr := errors.New("HTTPS server: open : no such file or directory")
		err := s.Start()
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%v)", expectedErr, err)
		}
		s.Stop()
		viper.Reset()
	}

	t.Log("HTTPS (no cert)")
	{
		viper.Set(config.KeySSLListen, ":65223")
		viper.Set(config.KeySSLCertFile, "testdata/missing.crt")
		s, _ := New(nil, nil)
		expectedErr := errors.New("HTTPS server: open testdata/missing.crt: no such file or directory")
		err := s.Start()
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%v)", expectedErr, err)
		}
		s.Stop()
		viper.Reset()
	}

	t.Log("HTTPS (no key)")
	{
		viper.Set(config.KeySSLListen, ":65224")
		viper.Set(config.KeySSLCertFile, "testdata/cert.crt")
		viper.Set(config.KeySSLKeyFile, "testdata/missing.key")
		s, _ := New(nil, nil)
		expectedErr := errors.New("HTTPS server: open testdata/missing.key: no such file or directory")
		err := s.Start()
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%v)", expectedErr, err)
		}
		s.Stop()
		viper.Reset()
	}

	t.Log("HTTPS cert/key fail")
	{
		viper.Set(config.KeySSLListen, ":65225")
		viper.Set(config.KeySSLCertFile, "testdata/cert.crt")
		viper.Set(config.KeySSLKeyFile, "testdata/key.key")
		s, _ := New(nil, nil)
		expectedErr := errors.New("HTTPS server: tls: failed to find any PEM data in certificate input")
		err := s.Start()
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%v)", expectedErr, err)
		}
		s.Stop()
		viper.Reset()
	}

	t.Log("Socket")
	{
		viper.Set(config.KeyListenSocketPath, "testdata/test.sock")
		s, _ := New(nil, nil)
		time.AfterFunc(1*time.Second, func() {
			s.Stop()
		})
		if err := s.Start(); err != nil {
			t.Fatalf("expected NO error, got (%v)", err)
		}
		viper.Reset()
	}

	t.Log("Socket w/bad config")
	{
		viper.Set(config.KeyListenSocketPath, "nodir/test.sock")
		s, _ := New(nil, nil)
		time.AfterFunc(1*time.Second, func() {
			s.Stop()
		})
		expect := errors.New("listen unix nodir/test.sock: bind: no such file or directory")
		err := s.Start()
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expect.Error() {
			t.Fatalf("expected (%s) got (%v)", expect, err)
		}
		viper.Reset()
	}
}
