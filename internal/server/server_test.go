// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"context"
	"errors"
	"path"
	"regexp"
	"runtime"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestNew(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Testing New w/HTTP")
	{
		t.Log("\tno config")
		{
			viper.Reset()
			ctx, cancel := context.WithCancel(context.Background())
			s, err := New(ctx, nil, nil, nil, nil)
			if err != nil {
				t.Fatalf("expected no error, got (%s)", err)
			}
			if len(s.svrHTTP) == 0 {
				t.Fatal("expected at least 1 http server")
			}
			cancel()
		}

		t.Log("\tempty config")
		{
			viper.Reset()
			viper.Set(config.KeyListen, []string{""})
			ctx, cancel := context.WithCancel(context.Background())
			s, err := New(ctx, nil, nil, nil, nil)
			if err != nil {
				t.Fatalf("expected no error, got (%s)", err)
			}
			if len(s.svrHTTP) == 0 {
				t.Fatal("expected at least 1 http server")
			}
			cancel()
		}

		t.Log("\tport config1 (colon)")
		{
			viper.Reset()
			viper.Set(config.KeyListen, []string{":2609"})
			ctx, cancel := context.WithCancel(context.Background())
			s, err := New(ctx, nil, nil, nil, nil)
			if err != nil {
				t.Fatalf("expected NO error, got (%s)", err)
			}
			if len(s.svrHTTP) == 0 {
				t.Fatal("expected at least 1 http server")
			}
			cancel()
		}

		t.Log("\tport config2 (no colon)")
		{
			viper.Reset()
			viper.Set(config.KeyListen, []string{"2609"})
			ctx, cancel := context.WithCancel(context.Background())
			s, err := New(ctx, nil, nil, nil, nil)
			if err != nil {
				t.Fatalf("expected NO error, got (%s)", err)
			}
			if len(s.svrHTTP) == 0 {
				t.Fatal("expected at least 1 http server")
			}
			expect := defaults.Listen
			if s.svrHTTP[0].address.String() != expect {
				t.Fatalf("expected (%s) got (%s)", expect, s.svrHTTP[0].address.String())
			}
			cancel()
		}

		t.Log("\taddress ipv4 config - invalid")
		{
			addr := "127.0.0.a"
			viper.Reset()
			viper.Set(config.KeyListen, []string{addr})
			ctx, cancel := context.WithCancel(context.Background())
			_, err := New(ctx, nil, nil, nil, nil)
			if err == nil {
				t.Fatal("expected error")
			}
			cancel()
		}

		t.Log("\taddress ipv4 config")
		{
			addr := "127.0.0.1"
			viper.Reset()
			viper.Set(config.KeyListen, []string{addr})
			ctx, cancel := context.WithCancel(context.Background())
			s, err := New(ctx, nil, nil, nil, nil)
			if err != nil {
				t.Fatalf("expected NO error, got (%s)", err)
			}
			if len(s.svrHTTP) == 0 {
				t.Fatal("expected at least 1 http server")
			}
			expect := addr + defaults.Listen
			if s.svrHTTP[0].address.String() != expect {
				t.Fatalf("expected (%s) got (%s)", expect, s.svrHTTP[0].address.String())
			}
			cancel()
		}

		t.Log("\taddress:port ipv4 config")
		{
			addr := "127.0.0.1:2610"
			viper.Reset()
			viper.Set(config.KeyListen, []string{addr})
			ctx, cancel := context.WithCancel(context.Background())
			s, err := New(ctx, nil, nil, nil, nil)
			if err != nil {
				t.Fatalf("expected NO error, got (%s)", err)
			}
			if len(s.svrHTTP) == 0 {
				t.Fatal("expected at least 1 http server")
			}
			expect := addr
			if s.svrHTTP[0].address.String() != expect {
				t.Fatalf("expected (%s) got (%s)", expect, s.svrHTTP[0].address.String())
			}
			cancel()
		}

		t.Log("\taddress ipv6 config - invalid format")
		{
			addr := "::1"
			viper.Reset()
			viper.Set(config.KeyListen, []string{addr})
			ctx, cancel := context.WithCancel(context.Background())
			_, err := New(ctx, nil, nil, nil, nil)
			if err == nil {
				t.Fatal("expected error")
			}
			cancel()
		}

		t.Log("\taddress ipv6 config - valid format")
		{
			addr := "[::1]"
			viper.Reset()
			viper.Set(config.KeyListen, []string{addr})
			ctx, cancel := context.WithCancel(context.Background())
			s, err := New(ctx, nil, nil, nil, nil)
			if err != nil {
				t.Fatalf("expected NO error, got (%s)", err)
			}
			if len(s.svrHTTP) == 0 {
				t.Fatal("expected at least 1 http server")
			}
			expect := addr + defaults.Listen
			if s.svrHTTP[0].address.String() != expect {
				t.Fatalf("expected (%s) got (%s)", expect, s.svrHTTP[0].address.String())
			}
			cancel()
		}

		t.Log("\taddress:port ipv6 config")
		{
			addr := "[::1]:2610"
			viper.Reset()
			viper.Set(config.KeyListen, []string{addr})
			ctx, cancel := context.WithCancel(context.Background())
			s, err := New(ctx, nil, nil, nil, nil)
			if err != nil {
				t.Fatalf("expected NO error, got (%s)", err)
			}
			if len(s.svrHTTP) == 0 {
				t.Fatal("expected at least 1 http server")
			}
			expect := addr
			if s.svrHTTP[0].address.String() != expect {
				t.Fatalf("expected (%s) got (%s)", expect, s.svrHTTP[0].address.String())
			}
			cancel()
		}

		t.Log("\tfqdn config - unknown name")
		{
			addr := "foo.bar"
			viper.Reset()
			viper.Set(config.KeyListen, []string{addr})
			ctx, cancel := context.WithCancel(context.Background())
			_, err := New(ctx, nil, nil, nil, nil)
			if err == nil {
				t.Fatal("expected error")
			}
			cancel()
		}

		t.Log("\tfqdn config")
		{
			// simply verifying it can resolve a FQDN to an IP address correctly
			addr := "www.google.com"
			viper.Reset()
			viper.Set(config.KeyListen, []string{addr})
			ctx, cancel := context.WithCancel(context.Background())
			s, err := New(ctx, nil, nil, nil, nil)
			if err != nil {
				t.Fatalf("expected NO error, got (%s)", err)
			}
			if len(s.svrHTTP) == 0 {
				t.Fatal("expected at least 1 http server")
			}
			if ok, _ := regexp.MatchString(`^\d{1,3}(\.\d{1,3}){3}:[0-9]+$`, s.svrHTTP[0].address.String()); !ok {
				t.Fatalf("expected (ipv4:port) got (%s)", s.svrHTTP[0].address.String())
			}
			cancel()
		}
	}

	t.Log("Tetsting New w/HTTPS")
	{
		t.Log("\taddress, no cert/key")
		{
			viper.Reset()
			viper.Set(config.KeySSLListen, ":2610")
			ctx, cancel := context.WithCancel(context.Background())
			_, err := New(ctx, nil, nil, nil, nil)
			if err == nil {
				t.Fatal("expecting error")
			}
			cancel()
		}

		t.Log("\taddress, bad cert, no key")
		{
			viper.Reset()
			viper.Set(config.KeySSLListen, ":2610")
			viper.Set(config.KeySSLCertFile, "testdata/missing.crt")
			ctx, cancel := context.WithCancel(context.Background())
			_, err := New(ctx, nil, nil, nil, nil)
			if err == nil {
				t.Fatal("expecting error")
			}
			cancel()
		}

		t.Log("\taddress, cert, no key")
		{
			viper.Reset()
			viper.Set(config.KeySSLListen, ":2610")
			viper.Set(config.KeySSLCertFile, "testdata/cert.crt")
			ctx, cancel := context.WithCancel(context.Background())
			_, err := New(ctx, nil, nil, nil, nil)
			if err == nil {
				t.Fatal("expecting error")
			}
			cancel()
		}

		t.Log("\taddress, cert, bad key")
		{
			viper.Reset()
			viper.Set(config.KeySSLListen, ":2610")
			viper.Set(config.KeySSLCertFile, "testdata/cert.crt")
			viper.Set(config.KeySSLKeyFile, "testdata/missing.key")
			ctx, cancel := context.WithCancel(context.Background())
			_, err := New(ctx, nil, nil, nil, nil)
			if err == nil {
				t.Fatal("expecting error")
			}
			cancel()
		}
	}

	if runtime.GOOS != "windows" {
		t.Log("Testing New w/Socket")
		{
			t.Log("\tw/config (file exists)")
			{
				viper.Reset()
				viper.Set(config.KeyListenSocket, []string{"testdata/exists.sock"})
				ctx, cancel := context.WithCancel(context.Background())
				_, err := New(ctx, nil, nil, nil, nil)
				if err == nil {
					t.Fatal("expected error")
				}
				cancel()
			}

			t.Log("\tw/valid config")
			{
				viper.Reset()
				viper.Set(config.KeyListenSocket, []string{path.Join("testdata", "test.sock")})
				ctx, cancel := context.WithCancel(context.Background())
				s, err := New(ctx, nil, nil, nil, nil)
				if err != nil {
					t.Fatalf("expected no error, got (%s)", err)
				}
				if len(s.svrSockets) != 1 {
					t.Fatal("expected 1 sockets")
				}
				s.svrSockets[0].listener.Close()
				cancel()
			}
		}
	}
}

func TestStartHTTP(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Testing startHTTP")

	t.Log("\tno config")
	{
		viper.Reset()
		s := &Server{}
		err := s.startHTTP(&httpServer{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("\tw/config")
	{
		viper.Reset()
		viper.Set(config.KeyListen, []string{":65111"})
		ctx, cancel := context.WithCancel(context.Background())
		s, err := New(ctx, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		if len(s.svrHTTP) != 1 {
			t.Fatal("expected 1 server")
		}
		time.AfterFunc(1*time.Second, func() {
			s.svrHTTP[0].server.Close()
			cancel()
		})
		if err := s.startHTTP(s.svrHTTP[0]); err != nil {
			t.Fatalf("expected NO error, got (%v)", err)
		}
	}
}

func TestStartHTTPS(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Testing startHTTPS")

	t.Log("\tno config")
	{
		viper.Reset()
		s := &Server{}
		err := s.startHTTPS()
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("\tw/config (empty cert)")
	{
		viper.Reset()
		viper.Set(config.KeySSLListen, ":65225")
		viper.Set(config.KeySSLCertFile, "testdata/cert.crt")
		viper.Set(config.KeySSLKeyFile, "testdata/key.key")
		ctx, cancel := context.WithCancel(context.Background())
		s, err := New(ctx, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		if err := s.startHTTPS(); err == nil {
			t.Fatal("expected error")
		}
		cancel()
	}
}

func TestStartSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets not available on " + runtime.GOOS)
		return
	}

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Testing startSocket")

	t.Log("\tno config")
	{
		viper.Reset()
		s := &Server{}
		err := s.startSocket(&socketServer{})
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}

	t.Log("\tw/bad config - invalid file")
	{
		viper.Reset()
		viper.Set(config.KeyListenSocket, []string{"nodir/test.sock"})
		ctx, cancel := context.WithCancel(context.Background())
		_, err := New(ctx, nil, nil, nil, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		cancel()
	}

	t.Log("\tw/config (server close)")
	{
		viper.Reset()
		viper.Set(config.KeyListenSocket, []string{path.Join("testdata", "test.sock")})
		ctx, cancel := context.WithCancel(context.Background())
		s, err := New(ctx, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		if len(s.svrSockets) != 1 {
			t.Fatal("expected 1 socket")
		}
		time.AfterFunc(1*time.Second, func() {
			s.svrSockets[0].server.Close()
			cancel()
		})
		serr := s.startSocket(s.svrSockets[0])
		if serr != nil {
			t.Fatalf("expected NO error, got (%v)", serr)
		}
	}

	t.Log("\tw/config (listener close)")
	{
		viper.Reset()
		viper.Set(config.KeyListenSocket, []string{path.Join("testdata", "test.sock")})
		ctx, cancel := context.WithCancel(context.Background())
		s, err := New(ctx, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		if len(s.svrSockets) != 1 {
			t.Fatal("expected 1 socket")
		}
		time.AfterFunc(1*time.Second, func() {
			s.svrSockets[0].listener.Close()
			cancel()
		})
		serr := s.startSocket(s.svrSockets[0])
		if serr == nil {
			t.Fatal("expected error")
		}
	}
}

func TestStart(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Testing Start")

	t.Log("\tno servers")
	{
		viper.Reset()
		ctx, cancel := context.WithCancel(context.Background())
		s, err := New(ctx, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		if len(s.svrHTTP) == 0 {
			t.Fatal("expected at least 1 http server")
		}
		cancel()
	}

	t.Log("\tvalid http, invalid https")
	{
		viper.Reset()
		viper.Set(config.KeyListen, []string{":65226"})
		viper.Set(config.KeySSLListen, ":65227")
		viper.Set(config.KeySSLCertFile, "testdata/cert.crt")
		viper.Set(config.KeySSLKeyFile, "testdata/key.key")
		ctx, cancel := context.WithCancel(context.Background())
		s, err := New(ctx, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		time.AfterFunc(1*time.Second, func() {
			cancel()
		})
		serr := s.Start()
		if serr == nil {
			t.Fatal("expected error")
		}
		expected := errors.New("SSL server: tls: failed to find any PEM data in certificate input")
		if serr.Error() != expected.Error() {
			t.Fatalf("expected (%s) got (%s)", expected, serr)
		}
	}
}

func TestStop(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Testing Stop")

	t.Run("no servers", func(t *testing.T) {
		viper.Reset()
		ctx, cancel := context.WithCancel(context.Background())
		s, err := New(ctx, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		if len(s.svrHTTP) == 0 {
			t.Fatal("expected at least 1 http server")
		}
		cancel()
	})

	t.Run("valid http, no socket", func(t *testing.T) {
		viper.Reset()
		viper.Set(config.KeyListen, []string{":65226"})
		ctx, cancel := context.WithCancel(context.Background())
		s, err := New(ctx, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}

		time.AfterFunc(1*time.Second, func() {
			s.Stop()
			cancel()
		})

		serr := s.Start()
		if serr != nil {
			t.Fatalf("expected no error, got (%s)", serr)
		}
	})

	if runtime.GOOS != "windows" {
		t.Run("valid http, valid socket", func(t *testing.T) {
			viper.Reset()
			viper.Set(config.KeyListen, []string{"localhost:"})
			viper.Set(config.KeyListenSocket, path.Join("testdata", "test.sock"))
			ctx, cancel := context.WithCancel(context.Background())
			s, err := New(ctx, nil, nil, nil, nil)
			if err != nil {
				t.Fatalf("expected no error, got (%s)", err)
			}

			time.AfterFunc(1*time.Second, func() {
				s.Stop()
				cancel()
			})

			serr := s.Start()
			if serr != nil {
				t.Fatalf("expected no error, got (%s)", serr)
			}
		})
	}
}
