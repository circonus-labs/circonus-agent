// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestNew(t *testing.T) {
	viper.Reset()
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Testing New w/HTTP")
	{
		t.Log("\tno config")
		{
			s, err := New(nil, nil, nil)
			if err != nil {
				t.Fatalf("expected no error, got (%s)", err)
			}
			if len(s.svrHTTP) == 0 {
				t.Fatal("expected at least 1 http server")
			}
		}

		t.Log("\tempty config")
		{
			viper.Set(config.KeyListen, []string{""})
			s, err := New(nil, nil, nil)
			if err != nil {
				t.Fatalf("expected no error, got (%s)", err)
			}
			if len(s.svrHTTP) == 0 {
				t.Fatal("expected at least 1 http server")
			}
			viper.Reset()
		}

		t.Log("\tport config1 (colon)")
		{
			viper.Set(config.KeyListen, []string{":2609"})
			s, err := New(nil, nil, nil)
			if err != nil {
				t.Fatalf("expected NO error, got (%s)", err)
			}
			if s == nil {
				t.Fatal("expected NOT nil")
			}
			if s.svrHTTP == nil {
				t.Fatal("expected NOT nil")
			}
			viper.Reset()
		}

		t.Log("\tport config2 (no colon)")
		{
			viper.Set(config.KeyListen, []string{"2609"})
			s, err := New(nil, nil, nil)
			if err != nil {
				t.Fatalf("expected NO error, got (%s)", err)
			}
			if s == nil {
				t.Fatal("expected NOT nil")
			}
			if s.svrHTTP == nil {
				t.Fatal("expected NOT nil")
			}
			expect := defaults.Listen
			if s.svrHTTP[0].address.String() != expect {
				t.Fatalf("expected (%s) got (%s)", expect, s.svrHTTP[0].address.String())
			}
			viper.Reset()
		}

		t.Log("\taddress ipv4 config - invalid")
		{
			addr := "127.0.0.a"
			viper.Set(config.KeyListen, []string{addr})
			_, err := New(nil, nil, nil)
			if err == nil {
				t.Fatal("expected error")
			}
			expect := "HTTP Server: resolving listen: lookup 127.0.0.a: no such host"
			if err.Error() != expect {
				t.Fatalf("expected (%s) got (%s)", expect, err)
			}
			viper.Reset()
		}

		t.Log("\taddress ipv4 config")
		{
			addr := "127.0.0.1"
			viper.Set(config.KeyListen, []string{addr})
			s, err := New(nil, nil, nil)
			if err != nil {
				t.Fatalf("expected NO error, got (%s)", err)
			}
			if s == nil {
				t.Fatal("expected NOT nil")
			}
			if s.svrHTTP == nil {
				t.Fatal("expected NOT nil")
			}
			expect := addr + defaults.Listen
			if s.svrHTTP[0].address.String() != expect {
				t.Fatalf("expected (%s) got (%s)", expect, s.svrHTTP[0].address.String())
			}
			viper.Reset()
		}

		t.Log("\taddress:port ipv4 config")
		{
			addr := "127.0.0.1:2610"
			viper.Set(config.KeyListen, []string{addr})
			s, err := New(nil, nil, nil)
			if err != nil {
				t.Fatalf("expected NO error, got (%s)", err)
			}
			if s == nil {
				t.Fatal("expected NOT nil")
			}
			if s.svrHTTP == nil {
				t.Fatal("expected NOT nil")
			}
			expect := addr
			if s.svrHTTP[0].address.String() != expect {
				t.Fatalf("expected (%s) got (%s)", expect, s.svrHTTP[0].address.String())
			}
			viper.Reset()
		}

		t.Log("\taddress ipv6 config - invalid format")
		{
			addr := "::1"
			viper.Set(config.KeyListen, []string{addr})
			_, err := New(nil, nil, nil)
			if err == nil {
				t.Fatal("expected error")
			}
			expect := "HTTP Server: parsing listen: address ::1: too many colons in address"
			if err.Error() != expect {
				t.Fatalf("expected (%s) got (%s)", expect, err)
			}
			viper.Reset()
		}

		t.Log("\taddress ipv6 config - valid format")
		{
			addr := "[::1]"
			viper.Set(config.KeyListen, []string{addr})
			s, err := New(nil, nil, nil)
			if err != nil {
				t.Fatalf("expected NO error, got (%s)", err)
			}
			if s == nil {
				t.Fatal("expected NOT nil")
			}
			if s.svrHTTP == nil {
				t.Fatal("expected NOT nil")
			}
			expect := addr + defaults.Listen
			if s.svrHTTP[0].address.String() != expect {
				t.Fatalf("expected (%s) got (%s)", expect, s.svrHTTP[0].address.String())
			}
			viper.Reset()
		}

		t.Log("\taddress:port ipv6 config")
		{
			addr := "[::1]:2610"
			viper.Set(config.KeyListen, []string{addr})
			s, err := New(nil, nil, nil)
			if err != nil {
				t.Fatalf("expected NO error, got (%s)", err)
			}
			if s == nil {
				t.Fatal("expected NOT nil")
			}
			if s.svrHTTP == nil {
				t.Fatal("expected NOT nil")
			}
			expect := addr
			if s.svrHTTP[0].address.String() != expect {
				t.Fatalf("expected (%s) got (%s)", expect, s.svrHTTP[0].address.String())
			}
			viper.Reset()
		}

		t.Log("\tfqdn config - unknown name")
		{
			addr := "foo.bar"
			viper.Set(config.KeyListen, []string{addr})
			_, err := New(nil, nil, nil)
			if err == nil {
				t.Fatal("expected error")
			}
			expect := "HTTP Server: resolving listen: lookup foo.bar: no such host"
			if err.Error() != expect {
				t.Fatalf("expected (%s) got (%s)", expect, err)
			}
			viper.Reset()
		}

		t.Log("\tfqdn config")
		{
			addr := "www.google.com"
			viper.Set(config.KeyListen, []string{addr})
			s, err := New(nil, nil, nil)
			if err != nil {
				t.Fatalf("expected NO error, got (%s)", err)
			}
			if s == nil {
				t.Fatal("expected NOT nil")
			}
			if s.svrHTTP == nil {
				t.Fatal("expected NOT nil")
			}
			if ok, _ := regexp.MatchString(`^\d{1,3}(\.\d{1,3}){3}:[0-9]+$`, s.svrHTTP[0].address.String()); !ok {
				t.Fatalf("expected (ipv4:port) got (%s)", s.svrHTTP[0].address.String())
			}
			viper.Reset()
		}
	}

	t.Log("Tetsting New w/HTTPS")
	{
		t.Log("\taddress, no cert/key")
		{
			viper.Set(config.KeySSLListen, ":2610")
			s, err := New(nil, nil, nil)
			if err == nil {
				t.Fatal("expecting error")
			}
			if s != nil {
				t.Fatal("expected nil")
			}
			expectedErr := errors.New("SSL server cert file: stat : no such file or directory")
			if err.Error() != expectedErr.Error() {
				t.Fatalf("expected (%s) got (%v)", expectedErr, err)
			}
			viper.Reset()
		}

		t.Log("\taddress, bad cert, no key")
		{
			viper.Set(config.KeySSLListen, ":2610")
			viper.Set(config.KeySSLCertFile, "testdata/missing.crt")
			s, err := New(nil, nil, nil)
			if err == nil {
				t.Fatal("expecting error")
			}
			if s != nil {
				t.Fatal("expected nil")
			}
			expectedErr := errors.New("SSL server cert file: stat testdata/missing.crt: no such file or directory")
			if err.Error() != expectedErr.Error() {
				t.Fatalf("expected (%s) got (%v)", expectedErr, err)
			}
			viper.Reset()
		}

		t.Log("\taddress, cert, no key")
		{
			viper.Set(config.KeySSLListen, ":2610")
			viper.Set(config.KeySSLCertFile, "testdata/cert.crt")
			s, err := New(nil, nil, nil)
			if err == nil {
				t.Fatal("expecting error")
			}
			if s != nil {
				t.Fatal("expected nil")
			}
			expectedErr := errors.New("SSL server key file: stat : no such file or directory")
			if err.Error() != expectedErr.Error() {
				t.Fatalf("expected (%s) got (%v)", expectedErr, err)
			}
			viper.Reset()
		}

		t.Log("\taddress, cert, bad key")
		{
			viper.Set(config.KeySSLListen, ":2610")
			viper.Set(config.KeySSLCertFile, "testdata/cert.crt")
			viper.Set(config.KeySSLKeyFile, "testdata/missing.key")
			s, err := New(nil, nil, nil)
			if err == nil {
				t.Fatal("expecting error")
			}
			if s != nil {
				t.Fatal("expected nil")
			}
			expectedErr := errors.New("SSL server key file: stat testdata/missing.key: no such file or directory")
			if err.Error() != expectedErr.Error() {
				t.Fatalf("expected (%s) got (%v)", expectedErr, err)
			}
			viper.Reset()
		}
	}

	t.Log("Testing New w/Socket")
	{
		t.Log("\tw/config (file exists)")
		{
			viper.Set(config.KeyListenSocket, []string{"testdata/exists.sock"})
			expected := errors.New("Socket server file (testdata/exists.sock) exists")
			_, err := New(nil, nil, nil)
			if err == nil {
				t.Fatal("expected error")
			}
			if err.Error() != expected.Error() {
				t.Fatalf("expected (%s) got (%s)", expected, err)
			}
			viper.Reset()
		}

		t.Log("\tw/valid config")
		{
			viper.Set(config.KeyListenSocket, []string{"testdata/test.sock"})
			s, err := New(nil, nil, nil)
			if err != nil {
				t.Fatalf("expected no error, got (%s)", err)
			}
			if len(s.svrSockets) != 1 {
				t.Fatal("expected 1 sockets")
			}
			s.svrSockets[0].listener.Close()
			viper.Reset()
		}
	}
}

func TestStartHTTP(t *testing.T) {
	viper.Reset()
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Testing startHTTP")

	t.Log("\tno config")
	{
		s := &Server{}
		err := s.startHTTP(&httpServer{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", nil)
		}
	}

	t.Log("\tw/config")
	{
		viper.Set(config.KeyListen, []string{":65111"})
		s, err := New(nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		if len(s.svrHTTP) != 1 {
			t.Fatal("expected 1 server")
		}
		time.AfterFunc(1*time.Second, func() {
			s.svrHTTP[0].server.Close()
		})
		if err := s.startHTTP(s.svrHTTP[0]); err != nil {
			t.Fatalf("expected NO error, got (%v)", err)
		}
		viper.Reset()
	}
}

func TestStartHTTPS(t *testing.T) {
	viper.Reset()
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Testing startHTTPS")

	t.Log("\tno config")
	{
		s := &Server{}
		err := s.startHTTPS()
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", nil)
		}
	}

	t.Log("\tw/config (empty cert)")
	{
		viper.Set(config.KeySSLListen, ":65225")
		viper.Set(config.KeySSLCertFile, "testdata/cert.crt")
		viper.Set(config.KeySSLKeyFile, "testdata/key.key")
		s, err := New(nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		if err := s.startHTTPS(); err == nil {
			t.Fatal("expected error")
		} else {
			expected := errors.New("SSL server: tls: failed to find any PEM data in certificate input")
			if err.Error() != expected.Error() {
				t.Fatalf("expected (%s) got (%s)", expected, err)
			}
		}
		viper.Reset()
	}
}

func TestStartSocket(t *testing.T) {
	viper.Reset()
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Testing startSocket")

	t.Log("\tno config")
	{
		s := &Server{}
		err := s.startSocket(&socketServer{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", nil)
		}
	}

	t.Log("\tw/bad config - invalid file")
	{
		viper.Set(config.KeyListenSocket, []string{"nodir/test.sock"})
		_, err := New(nil, nil, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		expect := errors.New("creating socket: listen unix nodir/test.sock: bind: no such file or directory")
		if err.Error() != expect.Error() {
			t.Fatalf("expected (%s) got (%v)", expect, err)
		}
		viper.Reset()
	}

	t.Log("\tw/config (server close)")
	{
		viper.Set(config.KeyListenSocket, []string{"testdata/test.sock"})
		s, err := New(nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		if len(s.svrSockets) != 1 {
			t.Fatal("expected 1 socket")
		}
		time.AfterFunc(1*time.Second, func() {
			s.svrSockets[0].server.Close()
		})
		serr := s.startSocket(s.svrSockets[0])
		if serr != nil {
			t.Fatalf("expected NO error, got (%v)", serr)
		}
		viper.Reset()
	}

	t.Log("\tw/config (listener close)")
	{
		viper.Set(config.KeyListenSocket, []string{"testdata/test.sock"})
		s, err := New(nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		if len(s.svrSockets) != 1 {
			t.Fatal("expected 1 socket")
		}
		time.AfterFunc(1*time.Second, func() {
			s.svrSockets[0].listener.Close()
		})
		expect := errors.New("socket server: accept unix testdata/test.sock: use of closed network connection")
		serr := s.startSocket(s.svrSockets[0])
		if serr == nil {
			t.Fatal("expected error")
		}
		if serr.Error() != expect.Error() {
			t.Fatalf("expected (%s) got (%v)", expect, serr)
		}

		viper.Reset()
	}
}

func TestStart(t *testing.T) {
	viper.Reset()
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Testing Start")

	t.Log("\tno servers")
	{
		s, err := New(nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		if len(s.svrHTTP) == 0 {
			t.Fatal("expected at least 1 http server")
		}
	}

	t.Log("\tvalid http, invalid https")
	{
		viper.Set(config.KeyListen, []string{":65226"})
		viper.Set(config.KeySSLListen, ":65227")
		viper.Set(config.KeySSLCertFile, "testdata/cert.crt")
		viper.Set(config.KeySSLKeyFile, "testdata/key.key")
		s, err := New(nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		serr := s.Start()
		if serr == nil {
			t.Fatal("expected error")
		}
		expected := errors.New("SSL server: tls: failed to find any PEM data in certificate input")
		if serr.Error() != expected.Error() {
			t.Fatalf("expected (%s) got (%s)", expected, serr)
		}
		viper.Reset()
	}
}

func TestStop(t *testing.T) {
	viper.Reset()
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Testing Stop")

	t.Log("\tno servers")
	{
		s, err := New(nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		if len(s.svrHTTP) == 0 {
			t.Fatal("expected at least 1 http server")
		}
	}

	t.Log("\tvalid http, valid socket")
	{
		viper.Set(config.KeyListen, []string{":65226"})
		viper.Set(config.KeyListenSocket, "testdata/test.sock")
		s, err := New(nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}

		time.AfterFunc(2*time.Second, func() {
			s.Stop()
		})

		serr := s.Start()
		if serr != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}

		viper.Reset()
	}
}
