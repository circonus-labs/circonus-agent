// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
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
			viper.Reset()
			viper.Set(config.KeyListen, []string{""})
			s, err := New(nil, nil, nil)
			if err != nil {
				t.Fatalf("expected no error, got (%s)", err)
			}
			if len(s.svrHTTP) == 0 {
				t.Fatal("expected at least 1 http server")
			}
		}

		t.Log("\tport config1 (colon)")
		{
			viper.Reset()
			viper.Set(config.KeyListen, []string{":2609"})
			s, err := New(nil, nil, nil)
			if err != nil {
				t.Fatalf("expected NO error, got (%s)", err)
			}
			if len(s.svrHTTP) == 0 {
				t.Fatal("expected at least 1 http server")
			}
		}

		t.Log("\tport config2 (no colon)")
		{
			viper.Reset()
			viper.Set(config.KeyListen, []string{"2609"})
			s, err := New(nil, nil, nil)
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
		}

		t.Log("\taddress ipv4 config - invalid")
		{
			addr := "127.0.0.a"
			viper.Reset()
			viper.Set(config.KeyListen, []string{addr})
			_, err := New(nil, nil, nil)
			if err == nil {
				t.Fatal("expected error")
			}
		}

		t.Log("\taddress ipv4 config")
		{
			addr := "127.0.0.1"
			viper.Reset()
			viper.Set(config.KeyListen, []string{addr})
			s, err := New(nil, nil, nil)
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
		}

		t.Log("\taddress:port ipv4 config")
		{
			addr := "127.0.0.1:2610"
			viper.Reset()
			viper.Set(config.KeyListen, []string{addr})
			s, err := New(nil, nil, nil)
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
		}

		t.Log("\taddress ipv6 config - invalid format")
		{
			addr := "::1"
			viper.Reset()
			viper.Set(config.KeyListen, []string{addr})
			_, err := New(nil, nil, nil)
			if err == nil {
				t.Fatal("expected error")
			}
		}

		t.Log("\taddress ipv6 config - valid format")
		{
			addr := "[::1]"
			viper.Reset()
			viper.Set(config.KeyListen, []string{addr})
			s, err := New(nil, nil, nil)
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
		}

		t.Log("\taddress:port ipv6 config")
		{
			addr := "[::1]:2610"
			viper.Reset()
			viper.Set(config.KeyListen, []string{addr})
			s, err := New(nil, nil, nil)
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
		}

		t.Log("\tfqdn config - unknown name")
		{
			addr := "foo.bar"
			viper.Reset()
			viper.Set(config.KeyListen, []string{addr})
			_, err := New(nil, nil, nil)
			if err == nil {
				t.Fatal("expected error")
			}
		}

		t.Log("\tfqdn config")
		{
			// simply verifying it can resolve a FQDN to an IP address correctly
			addr := "www.google.com"
			viper.Reset()
			viper.Set(config.KeyListen, []string{addr})
			s, err := New(nil, nil, nil)
			if err != nil {
				t.Fatalf("expected NO error, got (%s)", err)
			}
			if len(s.svrHTTP) == 0 {
				t.Fatal("expected at least 1 http server")
			}
			if ok, _ := regexp.MatchString(`^\d{1,3}(\.\d{1,3}){3}:[0-9]+$`, s.svrHTTP[0].address.String()); !ok {
				t.Fatalf("expected (ipv4:port) got (%s)", s.svrHTTP[0].address.String())
			}
		}
	}

	t.Log("Tetsting New w/HTTPS")
	{
		t.Log("\taddress, no cert/key")
		{
			viper.Reset()
			viper.Set(config.KeySSLListen, ":2610")
			_, err := New(nil, nil, nil)
			if err == nil {
				t.Fatal("expecting error")
			}
		}

		t.Log("\taddress, bad cert, no key")
		{
			viper.Reset()
			viper.Set(config.KeySSLListen, ":2610")
			viper.Set(config.KeySSLCertFile, "testdata/missing.crt")
			_, err := New(nil, nil, nil)
			if err == nil {
				t.Fatal("expecting error")
			}
		}

		t.Log("\taddress, cert, no key")
		{
			viper.Reset()
			viper.Set(config.KeySSLListen, ":2610")
			viper.Set(config.KeySSLCertFile, "testdata/cert.crt")
			_, err := New(nil, nil, nil)
			if err == nil {
				t.Fatal("expecting error")
			}
		}

		t.Log("\taddress, cert, bad key")
		{
			viper.Reset()
			viper.Set(config.KeySSLListen, ":2610")
			viper.Set(config.KeySSLCertFile, "testdata/cert.crt")
			viper.Set(config.KeySSLKeyFile, "testdata/missing.key")
			_, err := New(nil, nil, nil)
			if err == nil {
				t.Fatal("expecting error")
			}
		}
	}

	if runtime.GOOS != "windows" {
		t.Log("Testing New w/Socket")
		{
			t.Log("\tw/config (file exists)")
			{
				viper.Reset()
				viper.Set(config.KeyListenSocket, []string{"testdata/exists.sock"})
				_, err := New(nil, nil, nil)
				if err == nil {
					t.Fatal("expected error")
				}
			}

			t.Log("\tw/valid config")
			{
				viper.Reset()
				viper.Set(config.KeyListenSocket, []string{path.Join("testdata", "test.sock")})
				s, err := New(nil, nil, nil)
				if err != nil {
					t.Fatalf("expected no error, got (%s)", err)
				}
				if len(s.svrSockets) != 1 {
					t.Fatal("expected 1 sockets")
				}
				s.svrSockets[0].listener.Close()
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
			t.Fatalf("expected NO error, got (%s)", nil)
		}
	}

	done := make(chan int)
	t.Log("\tw/config")
	{
		viper.Reset()
		viper.Set(config.KeyListen, []string{":65111"})
		s, err := New(nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
			done <- 1
		}
		if len(s.svrHTTP) != 1 {
			t.Fatal("expected 1 server")
			done <- 1
		}
		time.AfterFunc(1*time.Second, func() {
			s.svrHTTP[0].server.Close()
			done <- 1
		})
		if err := s.startHTTP(s.svrHTTP[0]); err != nil {
			t.Fatalf("expected NO error, got (%v)", err)
			done <- 1
		}
	}
	<-done
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
			t.Fatalf("expected NO error, got (%s)", nil)
		}
	}

	t.Log("\tw/config (empty cert)")
	{
		viper.Reset()
		viper.Set(config.KeySSLListen, ":65225")
		viper.Set(config.KeySSLCertFile, "testdata/cert.crt")
		viper.Set(config.KeySSLKeyFile, "testdata/key.key")
		s, err := New(nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		if err := s.startHTTPS(); err == nil {
			t.Fatal("expected error")
		}
	}
}

func TestStartSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets not availble on " + runtime.GOOS)
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
			t.Fatalf("expected NO error, got (%s)", nil)
		}
	}

	t.Log("\tw/bad config - invalid file")
	{
		viper.Reset()
		viper.Set(config.KeyListenSocket, []string{"nodir/test.sock"})
		_, err := New(nil, nil, nil)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	done := make(chan int)

	t.Log("\tw/config (server close)")
	{
		viper.Reset()
		viper.Set(config.KeyListenSocket, []string{path.Join("testdata", "test.sock")})
		s, err := New(nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
			done <- 1
		}
		if len(s.svrSockets) != 1 {
			t.Fatal("expected 1 socket")
			done <- 1
		}
		time.AfterFunc(1*time.Second, func() {
			s.svrSockets[0].server.Close()
			done <- 1
		})
		serr := s.startSocket(s.svrSockets[0])
		if serr != nil {
			t.Fatalf("expected NO error, got (%v)", serr)
			done <- 1
		}
	}
	<-done

	t.Log("\tw/config (listener close)")
	{
		viper.Reset()
		viper.Set(config.KeyListenSocket, []string{path.Join("testdata", "test.sock")})
		s, err := New(nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
			done <- 1
		}
		if len(s.svrSockets) != 1 {
			t.Fatal("expected 1 socket")
			done <- 1
		}
		time.AfterFunc(1*time.Second, func() {
			s.svrSockets[0].listener.Close()
			done <- 1
		})
		serr := s.startSocket(s.svrSockets[0])
		if serr == nil {
			t.Fatal("expected error")
			done <- 1
		}
	}
	<-done
}

func TestStart(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Testing Start")

	t.Log("\tno servers")
	{
		viper.Reset()
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
		viper.Reset()
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
	}
}

func TestStop(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Testing Stop")

	t.Run("no servers", func(t *testing.T) {
		viper.Reset()
		s, err := New(nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		if len(s.svrHTTP) == 0 {
			t.Fatal("expected at least 1 http server")
		}
	})

	t.Run("valid http, no socket", func(t *testing.T) {
		done := make(chan int)
		viper.Reset()
		viper.Set(config.KeyListen, []string{":65226"})
		s, err := New(nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
			done <- 1
		}

		time.AfterFunc(2*time.Second, func() {
			s.Stop()
			done <- 1
		})

		serr := s.Start()
		if serr != nil {
			t.Fatalf("expected no error, got (%s)", serr)
			done <- 1
		}
		<-done
	})

	if runtime.GOOS != "windows" {
		t.Run("valid http, valid socket", func(t *testing.T) {
			done := make(chan int)
			viper.Reset()
			viper.Set(config.KeyListen, []string{"localhost:"})
			viper.Set(config.KeyListenSocket, path.Join("testdata", "test.sock"))
			s, err := New(nil, nil, nil)
			if err != nil {
				t.Fatalf("expected no error, got (%s)", err)
				done <- 1
			}

			time.AfterFunc(2*time.Second, func() {
				s.Stop()
				done <- 1
			})

			serr := s.Start()
			if serr != nil {
				t.Fatalf("expected no error, got (%s)", serr)
				done <- 1
			}
			<-done
		})
	}
}
