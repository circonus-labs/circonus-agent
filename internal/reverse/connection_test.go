// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/check"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

var tcert = []byte(`
-----BEGIN CERTIFICATE-----
MIICIDCCAYmgAwIBAgIQV//cFH6BSBmuGgMSXpBJUjANBgkqhkiG9w0BAQsFADAS
MRAwDgYDVQQKEwdBY21lIENvMCAXDTcwMDEwMTAwMDAwMFoYDzIwODQwMTI5MTYw
MDAwWjASMRAwDgYDVQQKEwdBY21lIENvMIGfMA0GCSqGSIb3DQEBAQUAA4GNADCB
iQKBgQDV5Czuhunu1X06LxwEKAa2gJD9O1r8gqjVxOr+gHElEjdHD9x4Zv3J9/T0
kZg6ztjhA6Vx1FPgqxjcQCoXeY6Bq0c3JybvONgY4v1MSGjdDqEY9RuyE44ziQ9w
+AXP3saX5WfhKrmIGjYLAeDwnLZg+hfyeD7AyWqtYs6EO3xgGQIDAQABo3UwczAO
BgNVHQ8BAf8EBAMCAqQwEwYDVR0lBAwwCgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUw
AwEB/zA7BgNVHREENDAyghhmYWtlLmNpcmNvbnVzLWJyb2tlci5jb22HBH8AAAGH
EAAAAAAAAAAAAAAAAAAAAAEwDQYJKoZIhvcNAQELBQADgYEATEUje+WIQowB4OeZ
DQEuRmtCiT9O1FXFj53lFswxrQ7+2BMpWZ2WB1aksoIMx0hfzkVsYTWSj02A/tyA
GUyo2Cii3gEhXYzwMuwTJpV4BL2Hkbp4/KhEXlFpIx/3VVQtURLWZOL+e42F1xVS
B3ufEbvf6JQzNaqNb22+SC0Uxzg=
-----END CERTIFICATE-----`)

var tkey = []byte(`
-----BEGIN RSA PRIVATE KEY-----
MIICXQIBAAKBgQDV5Czuhunu1X06LxwEKAa2gJD9O1r8gqjVxOr+gHElEjdHD9x4
Zv3J9/T0kZg6ztjhA6Vx1FPgqxjcQCoXeY6Bq0c3JybvONgY4v1MSGjdDqEY9Ruy
E44ziQ9w+AXP3saX5WfhKrmIGjYLAeDwnLZg+hfyeD7AyWqtYs6EO3xgGQIDAQAB
AoGAdCuhB9B5AEIt6MsrcUp4EumTVibFzT3+C1UPuTjzuwnAwjToLvDpSKgHAEqP
nuk3vEbptgB3qh/jucST1/oqnl7Bs9/2IxndiJZD5NocMOseKsLeelCT2YgjV1mP
PObVt/4vnqciC9wb+m723ZyWE/SdrbdrbVhfyOeb7ePPEz0CQQDoS4Eu5zpdBOD7
N0sEMMPXTKyTlaE8LC+//7DMDnFosKBDWNL4vrb0vB2061BX66fpIxiBIjnd9zcT
LrR71WPHAkEA67fkiIVNn9ijTiX++rk7JVeH+9HRah0BrZ/4AQ3xAWBc4gyL7YzB
7Uzxs9qD4sCixBc77VYhLs2pmDFlYWpdHwJBAKQJBX1gjWc4VcMwZYndAb6ch1Vk
mUoLjeCQJ4HBRTZ/W3yTUc+TpVCnMncaoE6lu5m3TcuKpsBmnX6vQYYcxusCQDcA
RbcFQ8OUjSZi/0gJiJ+B+RztLGwSMJ4OwZOdaSrlDUdBnjTjryxr08ofpr52LISM
11Ld0ghVvMjiXcGJTAkCQQCRFbMbKtfwtE8usCPimIdDSvyhfvYVC9Ye9ElE0TkF
XBI7D0Pg1nkGkRza/bcVUAUbDN9r3+eQKyruJSc/hsWV
-----END RSA PRIVATE KEY-----`)

func TestConnect(t *testing.T) {
	t.Log("Testing connect")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	cert, err := tls.X509KeyPair(tcert, tkey)
	if err != nil {
		t.Fatalf("expected no error, got (%s)", err)
	}

	tcfg := new(tls.Config)
	tcfg.Certificates = []tls.Certificate{cert}

	cp := x509.NewCertPool()
	clicert, err := x509.ParseCertificate(tcfg.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatalf("expected no error, got (%s)", err)
	}
	cp.AddCert(clicert)

	t.Log("valid")
	{
		l, err := tls.Listen("tcp", "127.0.0.1:0", tcfg)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		defer l.Close()

		go func() {
			conn, cerr := l.Accept()
			if cerr != nil {
				panic(cerr)
			}
			go func(c net.Conn) {
				io.Copy(c, c)
				c.Close()
			}(conn)
		}()

		chk, cerr := check.New(nil)
		if cerr != nil {
			t.Fatalf("expected no error, got (%s)", cerr)
		}
		s, err := New(chk, defaults.Listen)
		if err != nil {
			t.Fatalf("expected no error got (%s)", err)
		}

		s.tlsConfig = &tls.Config{
			RootCAs: cp,
		}

		tsURL, err := url.Parse("http://" + l.Addr().String() + "/check/foo-bar-baz#abc123")
		if err != nil {
			t.Fatalf("expected no error got (%s)", err)
		}

		s.reverseURL = tsURL
		s.dialerTimeout = 2 * time.Second

		if err := s.connect(); err != nil {
			t.Fatalf("expected no error got (%s)", err)
		}
		s.Stop()
	}

	t.Log("timeout")
	{
		l, err := tls.Listen("tcp", "127.0.0.1:0", tcfg)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		defer l.Close()

		// basically, just don't accept any connections

		chk, cerr := check.New(nil)
		if cerr != nil {
			t.Fatalf("expected no error, got (%s)", cerr)
		}
		s, err := New(chk, defaults.Listen)
		if err != nil {
			t.Fatalf("expected no error got (%s)", err)
		}

		s.tlsConfig = &tls.Config{
			RootCAs: cp,
		}

		tsURL, err := url.Parse("http://" + l.Addr().String() + "/check/foo-bar-baz#abc123")
		if err != nil {
			t.Fatalf("expected no error got (%s)", err)
		}

		s.reverseURL = tsURL
		s.dialerTimeout = 2 * time.Second

		expect := errors.Errorf("connecting to %s: tls: DialWithDialer timed out", l.Addr().String())
		connErr := s.connect()
		if connErr == nil {
			t.Fatal("expected error")
		}
		if connErr.Error() != expect.Error() {
			t.Fatalf("expected (%s) got (%s)", expect, connErr)
		}
		s.Stop()
	}

	t.Log("error (closed connection)")
	{
		l, err := tls.Listen("tcp", "127.0.0.1:0", tcfg)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		defer l.Close()

		go func() {
			conn, cerr := l.Accept()
			if cerr != nil {
				panic(cerr)
			}
			go func(c net.Conn) {
				c.Close()
			}(conn)
		}()

		chk, cerr := check.New(nil)
		if cerr != nil {
			t.Fatalf("expected no error, got (%s)", cerr)
		}
		s, err := New(chk, defaults.Listen)
		if err != nil {
			t.Fatalf("expected no error got (%s)", err)
		}

		s.tlsConfig = &tls.Config{
			RootCAs: cp,
		}

		tsURL, err := url.Parse("http://" + l.Addr().String() + "/check/foo-bar-baz#abc123")
		if err != nil {
			t.Fatalf("expected no error got (%s)", err)
		}

		s.reverseURL = tsURL
		s.dialerTimeout = 2 * time.Second

		connErr := s.connect()
		if connErr == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(connErr.Error(), l.Addr().String()) {
			t.Fatalf("expected (%s) got (%s)", l.Addr().String(), connErr)
		}
		s.Stop()
	}

}

func TestSetNextDelay(t *testing.T) {
	t.Log("Testing setNextDelay")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("delay == max")
	{
		viper.Set(config.KeyReverse, false)
		chk, cerr := check.New(nil)
		if cerr != nil {
			t.Fatalf("expected no error, got (%s)", cerr)
		}
		c, err := New(chk, defaults.Listen)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}

		c.delay = c.maxDelay
		c.setNextDelay()
		if c.delay != c.maxDelay {
			t.Fatalf("delay changed, not set to max")
		}
	}

	t.Log("valid change")
	{
		viper.Set(config.KeyReverse, false)
		chk, cerr := check.New(nil)
		if cerr != nil {
			t.Fatalf("expected no error, got (%s)", cerr)
		}
		c, err := New(chk, defaults.Listen)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}

		currDelay := c.delay

		c.setNextDelay()

		if c.delay == currDelay {
			t.Fatalf("delay did NOT changed %s == %s", c.delay.String(), currDelay.String())
		}

		min := time.Duration(minDelayStep) * time.Second
		max := time.Duration(maxDelayStep) * time.Second
		diff := c.delay - currDelay

		if diff < min {
			t.Fatalf("delay increment (%s) < minimum (%s)", diff.String(), min.String())
		}

		if diff > max {
			t.Fatalf("delay increment (%s) > maximum (%s)", diff.String(), max.String())
		}
	}

	t.Log("reset to max")
	{
		viper.Set(config.KeyReverse, false)
		chk, cerr := check.New(nil)
		if cerr != nil {
			t.Fatalf("expected no error, got (%s)", cerr)
		}
		c, err := New(chk, defaults.Listen)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}

		c.delay = 61 * time.Second

		c.setNextDelay()

		if c.delay != c.maxDelay {
			t.Fatalf("delay did NOT reset %s == %s", c.delay.String(), c.maxDelay)
		}
	}
}

func TestResetConnectionAttempts(t *testing.T) {
	t.Log("Testing resetConnectionAttempts")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	viper.Set(config.KeyReverse, false)
	chk, cerr := check.New(nil)
	if cerr != nil {
		t.Fatalf("expected no error, got (%s)", cerr)
	}
	c, err := New(chk, defaults.Listen)
	if err != nil {
		t.Fatalf("expected no error, got (%s)", err)
	}

	c.delay = 10 * time.Second
	c.connAttempts = 10

	c.resetConnectionAttempts()

	if c.delay != 1*time.Second {
		t.Fatalf("delay not reset (%s)", c.delay.String())
	}

	if c.connAttempts != 0 {
		t.Fatalf("attempts not reset (%d)", c.connAttempts)
	}
}
