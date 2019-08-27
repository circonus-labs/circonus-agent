// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package connection

// import (
// 	"bytes"
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"net/http"
// 	"net/http/httptest"
// 	"net/url"
// 	"testing"
// 	"time"

// 	"github.com/circonus-labs/circonus-agent/internal/check"
// 	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
// 	"github.com/rs/zerolog"
// )

// func TestSendMetricData(t *testing.T) {
// 	t.Log("Testing sendMetricData")

// 	zerolog.SetGlobalLevel(zerolog.Disabled)

// 	data := []byte(`{"test":1}`)
// 	buff := bytes.NewBuffer([]byte{})

// 	chk, cerr := check.New(nil)
// 	if cerr != nil {
// 		t.Fatalf("expected no error, got (%s)", cerr)
// 	}
// 	ctx, cancel := context.WithCancel(context.Background())
// 	s, err := New(ctx, chk, defaults.Listen)
// 	if err != nil {
// 		t.Fatalf("expected no error, got (%s)", err)
// 	}

// 	err = s.sendMetricData(buff, 1, &data, time.Now())
// 	if err != nil {
// 		t.Fatalf("expected no error, got (%s)", err)
// 	}

// 	hdr, err := readFrameHeader(buff)
// 	if err != nil {
// 		t.Fatalf("expected no error, got (%s)", err)
// 	}
// 	if hdr == nil {
// 		t.Fatal("expected not nil")
// 	}
// 	if hdr.channelID != 1 {
// 		t.Fatalf("expected channel 1, got %v", hdr.channelID)
// 	}
// 	if hdr.payloadLen != uint32(len(data)) {
// 		t.Fatalf("expected expected len %d, got %v", len(data), hdr.payloadLen)
// 	}

// 	m, err := readFramePayload(buff, hdr.payloadLen)
// 	if err != nil {
// 		t.Fatalf("expected no error, got (%s)", err)
// 	}
// 	if m == nil {
// 		t.Fatal("expected not nil")
// 	}
// 	if !bytes.Equal(m, data) {
// 		t.Fatalf("expected (%s) got (%s)", string(data), string(m))
// 	}
// 	cancel()
// }

// func TestFetchMetricData(t *testing.T) {
// 	t.Log("Testing fetchMetricData")

// 	zerolog.SetGlobalLevel(zerolog.Disabled)

// 	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		w.Header().Set("Content-Type", "application/json")
// 		w.WriteHeader(http.StatusOK)
// 		metrics := map[string]int{"test": 1}
// 		if err := json.NewEncoder(w).Encode(metrics); err != nil {
// 			panic(err)
// 		}
// 	}))

// 	defer ts.Close()

// 	tsURL, err := url.Parse(ts.URL)
// 	if err != nil {
// 		t.Fatalf("expected no error, got %s", err)
// 	}

// 	chk, cerr := check.New(nil)
// 	if cerr != nil {
// 		t.Fatalf("expected no error, got (%s)", cerr)
// 	}
// 	ctx, cancel := context.WithCancel(context.Background())
// 	s, err := New(ctx, chk, defaults.Listen)
// 	if err != nil {
// 		t.Fatalf("expected no error, got (%s)", err)
// 	}
// 	s.agentAddress = tsURL.Host
// 	s.metricTimeout = 3 * time.Second

// 	req := []byte("GET / HTTP/1.1\r\nHost: " + s.agentAddress + "\r\n\r\n")
// 	time.AfterFunc(1*time.Second, func() {
// 		ts.CloseClientConnections()
// 	})
// 	data, err := s.fetchMetricData(&req, uint16(0))
// 	if err != nil {
// 		t.Fatalf("expected no error, got (%s) %#v", err, data)
// 	}
// 	if data == nil {
// 		t.Fatal("expected not nil")
// 	}

// 	if !bytes.Contains(*data, []byte(`{"test":1}`)) {
// 		fmt.Println(string(*data))
// 		t.Fatalf("%s", string(*data))
// 	}
// 	cancel()
// }
