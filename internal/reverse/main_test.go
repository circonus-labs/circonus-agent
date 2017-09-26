// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-gometrics/api"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

type pkicacert struct {
	Contents string `json:"contents"`
}

var (
	brokerSim       = httptest.NewTLSServer(http.HandlerFunc(brokerHandler))
	apiSim          = httptest.NewServer(http.HandlerFunc(apiHandler))
	testCheckBundle api.CheckBundle
	testBroker      api.Broker
	cacert          pkicacert
)

func init() {
	if data, err := ioutil.ReadFile("testdata/check1234.json"); err != nil {
		panic(err)
	} else {
		if err := json.Unmarshal(data, &testCheckBundle); err != nil {
			panic(err)
		}
	}

	if data, err := ioutil.ReadFile("testdata/broker1234.json"); err != nil {
		panic(err)
	} else {
		if err := json.Unmarshal(data, &testBroker); err != nil {
			panic(err)
		}
	}

	if data, err := ioutil.ReadFile("testdata/ca.crt"); err != nil {
		panic(err)
	} else {
		cacert.Contents = string(data)
	}
}

// brokerHandler simulates an actual broker
func brokerHandler(w http.ResponseWriter, r *http.Request) {
	// path := r.URL.Path
	// reqURL := r.URL.String()
	// fmt.Println(path, reqURL)
	w.WriteHeader(200)
	fmt.Fprintln(w, "")
}

// apiHandler simulates an api server for test requests
func apiHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	reqURL := r.URL.String()
	// fmt.Println(path, reqURL)
	switch path {
	case "/check_bundle":
		if strings.Contains(reqURL, "search") {
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(reqURL, "notfound") {
				w.WriteHeader(200)
				fmt.Fprintln(w, "[]")
			} else if strings.Contains(reqURL, "multiple") {
				c := []api.CheckBundle{api.CheckBundle{}, api.CheckBundle{}}
				ret, err := json.Marshal(c)
				if err != nil {
					panic(err)
				}
				w.WriteHeader(200)
				fmt.Fprintln(w, string(ret))
			} else if strings.Contains(reqURL, "error") {
				w.WriteHeader(500)
				fmt.Fprintln(w, `{"error":"requested an error"}`)
			} else if strings.Contains(reqURL, "test") {
				c := []api.CheckBundle{testCheckBundle}
				ret, err := json.Marshal(c)
				if err != nil {
					panic(err)
				}
				w.WriteHeader(200)
				fmt.Fprintln(w, string(ret))
			} else {
				w.WriteHeader(200)
				fmt.Fprintln(w, "[]")
			}
		}
	case "/check_bundle/1234":
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "application/json")
		ret, err := json.Marshal(testCheckBundle)
		if err != nil {
			panic(err)
		}
		fmt.Fprintln(w, string(ret))
	case "/check_bundle/5678":
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "application/json")
		cb := testCheckBundle
		cb.ReverseConnectURLs[0] = brokerSim.URL
		ret, err := json.Marshal(cb)
		if err != nil {
			panic(err)
		}
		fmt.Fprintln(w, string(ret))
	case "/broker/1234":
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "application/json")
		ret, err := json.Marshal(testBroker)
		if err != nil {
			panic(err)
		}
		fmt.Fprintln(w, string(ret))
	case "/pki/ca.crt":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		ret, err := json.Marshal(cacert)
		if err != nil {
			panic(err)
		}
		fmt.Fprintln(w, string(ret))
	default:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "not found")
	}
}

func TestStart(t *testing.T) {
	t.Log("Testing Start")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("Reverse disabled (no start)")
	{
		viper.Set(config.KeyReverse, false)
		c, err := New()
		viper.Reset()

		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}

		if c == nil {
			t.Fatal("expected not nil")
		}
	}

	t.Log("Reverse disabled (start)")
	{
		viper.Set(config.KeyReverse, false)
		c, err := New()
		viper.Reset()

		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}

		if c == nil {
			t.Fatal("expected not nil")
		}

		err = c.Start(context.Background())

		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
	}

	t.Log("No config")
	{
		viper.Set(config.KeyReverse, true)
		_, err := New()
		viper.Reset()

		expectedErr := errors.New("reverse configuration (check): Initializing cgm API: API Token is required")
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
	}

	ltFlag := "circonus-agent_LONG_TEST"
	if longTest, _ := strconv.ParseBool(os.Getenv(ltFlag)); !longTest {
		t.Logf("Skipping long tests, set %s=1 to enable", ltFlag)
		return
	}

	t.Logf("Testing failed conn attempts, expect success after %d attempts", maxConnRetry)

	zerolog.SetGlobalLevel(zerolog.DebugLevel) // provide some feedback to terminal

	t.Log("connection refused")
	{
		viper.Set(config.KeyReverse, true)
		viper.Set(config.KeyReverseCID, "1234")
		viper.Set(config.KeyAPITokenKey, "foo")
		viper.Set(config.KeyAPITokenApp, "foo")
		viper.Set(config.KeyAPIURL, apiSim.URL)
		c, err := New()
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		err = c.Start(context.Background())
		viper.Reset()

		expectedErr := errors.New("establishing reverse connection: dial tcp 127.0.0.1:1234: getsockopt: connection refused")
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected (%s) got (%s)", expectedErr, err)
		}
	}
}

func TestStop(t *testing.T) {
	t.Log("Testing Stop")

	t.Log("disabled")
	{
		viper.Set(config.KeyReverse, false)
		c, err := New()
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}

		c.Stop()
	}

	t.Log("nil conn")
	{
		viper.Set(config.KeyReverse, false)
		c, err := New()
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}

		c.enabled = true
		c.conn = nil

		c.Stop()
	}

}
