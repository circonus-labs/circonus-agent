// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

// import (
// 	"context"
// 	"crypto/tls"
// 	"crypto/x509"
// 	"encoding/json"
// 	"fmt"
// 	"io/ioutil"
// 	"net"
// 	"net/http"
// 	"net/http/httptest"
// 	"net/http/httputil"
// 	"net/url"
// 	"os"
// 	"strconv"
// 	"strings"
// 	"testing"
// 	"time"

// 	"github.com/circonus-labs/circonus-agent/internal/check"
// 	"github.com/circonus-labs/circonus-agent/internal/config"
// 	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
// 	"github.com/circonus-labs/go-apiclient"
// 	"github.com/pkg/errors"
// 	"github.com/rs/zerolog"
// 	"github.com/rs/zerolog/log"
// 	"github.com/spf13/viper"
// )

// type pkicacert struct {
// 	Contents string `json:"contents"`
// }

// var (
// 	brokerSim       = httptest.NewTLSServer(http.HandlerFunc(brokerHandler))
// 	apiSim          = httptest.NewServer(http.HandlerFunc(apiHandler))
// 	testCheckBundle apiclient.CheckBundle
// 	testBroker      apiclient.Broker
// 	cacert          pkicacert
// )

// func init() {
// 	if data, err := ioutil.ReadFile("testdata/check1234.json"); err != nil {
// 		panic(err)
// 	} else {
// 		if err := json.Unmarshal(data, &testCheckBundle); err != nil {
// 			panic(err)
// 		}
// 	}

// 	if data, err := ioutil.ReadFile("testdata/broker1234.json"); err != nil {
// 		panic(err)
// 	} else {
// 		if err := json.Unmarshal(data, &testBroker); err != nil {
// 			panic(err)
// 		}
// 	}

// 	if data, err := ioutil.ReadFile("testdata/ca.crt"); err != nil {
// 		panic(err)
// 	} else {
// 		cacert.Contents = string(data)
// 	}
// }

// func dumpReq(r *http.Request) error {
// 	output, err := httputil.DumpRequest(r, true)
// 	if err != nil {
// 		return errors.Wrap(err, "Error dumping request")
// 	}
// 	fmt.Println(string(output))
// 	return nil
// }

// // brokerHandler simulates an actual broker
// func brokerHandler(w http.ResponseWriter, r *http.Request) {
// 	// dumpReq(r)
// 	// path := r.URL.Path
// 	// reqURL := r.URL.String()
// 	w.WriteHeader(200)
// 	fmt.Fprintln(w, "")
// }

// // apiHandler simulates an api server for test requests
// func apiHandler(w http.ResponseWriter, r *http.Request) {
// 	// dumpReq(r)

// 	path := r.URL.Path
// 	reqURL := r.URL.String()

// 	switch path {
// 	case "/check_bundle":
// 		if strings.Contains(reqURL, "search") {
// 			w.Header().Set("Content-Type", "application/json")
// 			if strings.Contains(reqURL, "notfound") {
// 				w.WriteHeader(200)
// 				fmt.Fprintln(w, "[]")
// 			} else if strings.Contains(reqURL, "multiple") {
// 				c := []apiclient.CheckBundle{{}, {}}
// 				ret, err := json.Marshal(c)
// 				if err != nil {
// 					panic(err)
// 				}
// 				w.WriteHeader(200)
// 				fmt.Fprintln(w, string(ret))
// 			} else if strings.Contains(reqURL, "error") {
// 				w.WriteHeader(500)
// 				fmt.Fprintln(w, `{"error":"requested an error"}`)
// 			} else if strings.Contains(reqURL, "test") {
// 				c := []apiclient.CheckBundle{testCheckBundle}
// 				ret, err := json.Marshal(c)
// 				if err != nil {
// 					panic(err)
// 				}
// 				w.WriteHeader(200)
// 				fmt.Fprintln(w, string(ret))
// 			} else {
// 				w.WriteHeader(200)
// 				fmt.Fprintln(w, "[]")
// 			}
// 		}
// 	case "/check_bundle/1234":
// 		w.WriteHeader(200)
// 		w.Header().Set("Content-Type", "application/json")
// 		ret, err := json.Marshal(testCheckBundle)
// 		if err != nil {
// 			panic(err)
// 		}
// 		fmt.Fprintln(w, string(ret))
// 	case "/check_bundle/5678":
// 		w.WriteHeader(200)
// 		w.Header().Set("Content-Type", "application/json")
// 		cb := testCheckBundle
// 		cb.ReverseConnectURLs[0] = brokerSim.URL
// 		ret, err := json.Marshal(cb)
// 		if err != nil {
// 			panic(err)
// 		}
// 		fmt.Fprintln(w, string(ret))
// 	case "/broker/1234":
// 		w.WriteHeader(200)
// 		w.Header().Set("Content-Type", "application/json")
// 		ret, err := json.Marshal(testBroker)
// 		if err != nil {
// 			panic(err)
// 		}
// 		fmt.Fprintln(w, string(ret))
// 	case "/pki/ca.crt":
// 		w.Header().Set("Content-Type", "application/json")
// 		w.WriteHeader(200)
// 		ret, err := json.Marshal(cacert)
// 		if err != nil {
// 			panic(err)
// 		}
// 		fmt.Fprintln(w, string(ret))
// 	default:
// 		w.WriteHeader(http.StatusNotFound)
// 		fmt.Fprint(w, "not found")
// 	}
// }

// func TestNew(t *testing.T) {
// 	t.Log("Testing New")

// 	zerolog.SetGlobalLevel(zerolog.Disabled)
// 	var logger zerolog.Logger

// 	t.Log("Reverse disabled")
// 	{
// 		viper.Set(config.KeyReverse, false)
// 		chk, cerr := check.New(nil)
// 		if cerr != nil {
// 			t.Fatalf("expected no error, got (%s)", cerr)
// 		}
// 		c, err := New(logger, chk, defaults.Listen)
// 		viper.Reset()

// 		if err != nil {
// 			t.Fatalf("expected no error, got (%s)", err)
// 		}

// 		if c == nil {
// 			t.Fatal("expected not nil")
// 		}
// 		cancel()
// 	}

// 	t.Log("Reverse enabled (no config)")
// 	{
// 		viper.Set(config.KeyReverse, true)
// 		_, err := New(logger, nil, defaults.Listen)
// 		viper.Reset()

// 		if err == nil {
// 			t.Fatal("expected error")
// 		}
// 		if err.Error() != "invalid check value (empty)" {
// 			t.Fatalf("unexpected error (%s)", err)
// 		}
// 		cancel()
// 	}
// }

// func TestStart(t *testing.T) {
// 	t.Log("Testing Start")

// 	zerolog.SetGlobalLevel(zerolog.Disabled)
// 	var logger zerolog.Logger

// 	t.Log("Reverse disabled")
// 	{
// 		viper.Set(config.KeyReverse, false)
// 		chk, cerr := check.New(nil)
// 		if cerr != nil {
// 			t.Fatalf("expected no error, got (%s)", cerr)
// 		}
// 		c, err := New(logger, chk, defaults.Listen)
// 		viper.Reset()

// 		if err != nil {
// 			t.Fatalf("expected no error, got (%s)", err)
// 		}

// 		if c == nil {
// 			t.Fatal("expected not nil")
// 		}

// 		time.AfterFunc(2*time.Second, func() {
// 			cancel()
// 		})
// 		err = c.Start()

// 		if err != nil {
// 			t.Fatalf("expected no error, got (%s)", err)
// 		}
// 	}

// 	t.Log("valid")
// 	{
// 		cert, err := tls.X509KeyPair(tcert, tkey)
// 		if err != nil {
// 			t.Fatalf("expected no error, got (%s)", err)
// 		}

// 		tcfg := new(tls.Config)
// 		tcfg.Certificates = []tls.Certificate{cert}

// 		cp := x509.NewCertPool()
// 		clicert, err := x509.ParseCertificate(tcfg.Certificates[0].Certificate[0])
// 		if err != nil {
// 			t.Fatalf("expected no error, got (%s)", err)
// 		}
// 		cp.AddCert(clicert)

// 		l, err := tls.Listen("tcp", "127.0.0.1:0", tcfg)
// 		if err != nil {
// 			t.Fatalf("expected no error, got (%s)", err)
// 		}
// 		defer l.Close()

// 		go func() {
// 			conn, cerr := l.Accept()
// 			if cerr != nil {
// 				return
// 			}
// 			go func(c net.Conn) {
// 				var werr error
// 				_, werr = c.Write(buildFrame(1, true, []byte("CONNECT")))
// 				if werr != nil {
// 					panic(werr)
// 				}
// 				_, werr = c.Write(buildFrame(1, false, []byte{}))
// 				if werr != nil {
// 					panic(werr)
// 				}
// 				// leave open, broker connections are persistent
// 				// closing it will trigger reconnecting
// 				//c.Close()
// 			}(conn)
// 		}()

// 		viper.Set(config.KeyReverse, true)
// 		viper.Set(config.KeyCheckBundleID, "1234")
// 		viper.Set(config.KeyAPITokenKey, "foo")
// 		viper.Set(config.KeyAPITokenApp, "foo")
// 		viper.Set(config.KeyAPIURL, apiSim.URL)
// 		chk, cerr := check.New(nil)
// 		if cerr != nil {
// 			t.Fatalf("expected no error, got (%s)", cerr)
// 		}
// 		s, err := New(logger, chk, l.Addr().String())
// 		if err != nil {
// 			t.Fatalf("expected no error got (%s)", err)
// 		}

// 		tsURL, err := url.Parse("http://" + l.Addr().String() + "/check/foo-bar-baz#abc123")
// 		if err != nil {
// 			t.Fatalf("expected no error got (%s)", err)
// 		}
// 		ra, err := net.ResolveTCPAddr("tcp", tsURL.Host)
// 		if err != nil {
// 			t.Fatalf("expected no error got (%s)", err)
// 		}

// 		s.revConfig = check.ReverseConfig{
// 			ReverseURL: tsURL,
// 			BrokerAddr: ra,
// 			TLSConfig: &tls.Config{
// 				RootCAs: cp,
// 			},
// 		}

// 		s.dialerTimeout = 1 * time.Second

// 		time.AfterFunc(2*time.Second, func() {
// 			cancel()
// 		})

// 		if err := s.Start(); err != nil {
// 			if err.Error() != "Shutdown requested" {
// 				t.Fatalf("expected no error got (%s)", err)
// 			}
// 		}

// 		viper.Reset()
// 	}
// }

// func TestStartLong(t *testing.T) {
// 	ltFlag := "CIRCONUS_AGENT_LONG_TEST"
// 	if longTest, _ := strconv.ParseBool(os.Getenv(ltFlag)); !longTest {
// 		t.Logf("Skipping long tests, set %s=1 to enable", ltFlag)
// 		return
// 	}

// 	zerolog.SetGlobalLevel(zerolog.DebugLevel) // provide some feedback to terminal

// 	t.Log("connection refused")
// 	{
// 		viper.Set(config.KeyReverse, true)
// 		viper.Set(config.KeyCheckBundleID, "1234")
// 		viper.Set(config.KeyAPITokenKey, "foo")
// 		viper.Set(config.KeyAPITokenApp, "foo")
// 		viper.Set(config.KeyAPIURL, apiSim.URL)
// 		chk, cerr := check.New(nil)
// 		if cerr != nil {
// 			t.Fatalf("expected no error, got (%s)", cerr)
// 		}
// 		ctx, cancel := context.WithCancel(context.Background())
// 		c, err := New(ctx, chk, defaults.Listen)
// 		if err != nil {
// 			t.Fatalf("expected no error, got (%s)", err)
// 		}
// 		time.AfterFunc(1*time.Second, func() {
// 			cancel()
// 		})
// 		err = c.Start()
// 		viper.Reset()

// 		expectedErr := errors.New("establishing reverse connection: dial tcp 127.0.0.1:1234: getsockopt: connection refused")
// 		if err == nil {
// 			t.Fatal("expected error")
// 		}
// 		if err.Error() != expectedErr.Error() {
// 			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
// 		}
// 	}
// }

// func TestStop(t *testing.T) {
// 	t.Log("Testing Stop")

// 	zerolog.SetGlobalLevel(zerolog.Disabled)

// 	t.Log("disabled")
// 	{
// 		viper.Set(config.KeyReverse, false)
// 		chk, cerr := check.New(nil)
// 		if cerr != nil {
// 			t.Fatalf("expected no error, got (%s)", cerr)
// 		}
// 		ctx, cancel := context.WithCancel(context.Background())
// 		_, err := New(ctx, chk, defaults.Listen)
// 		if err != nil {
// 			t.Fatalf("expected no error, got (%s)", err)
// 		}
// 		cancel()
// 	}

// 	t.Log("nil conn")
// 	{
// 		viper.Set(config.KeyReverse, false)
// 		chk, cerr := check.New(nil)
// 		if cerr != nil {
// 			t.Fatalf("expected no error, got (%s)", cerr)
// 		}
// 		ctx, cancel := context.WithCancel(context.Background())
// 		c, err := New(ctx, chk, defaults.Listen)
// 		if err != nil {
// 			t.Fatalf("expected no error, got (%s)", err)
// 		}

// 		c.enabled = true
// 		cancel()
// 	}

// }
