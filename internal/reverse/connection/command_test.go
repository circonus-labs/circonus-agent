// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package connection

// import (
// 	"bytes"
// 	"context"
// 	"errors"
// 	"fmt"
// 	"net/http"
// 	"net/http/httptest"
// 	"testing"

// 	"github.com/circonus-labs/circonus-agent/internal/check"
// 	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
// 	"github.com/rs/zerolog"
// )

// func TestReadCommand(t *testing.T) {
// 	t.Log("Testing readCommand")

// 	zerolog.SetGlobalLevel(zerolog.Disabled)

// 	tests := []struct {
// 		name         string
// 		commandFrame []byte
// 		payloadFrame []byte
// 		shouldError  bool
// 		err          error
// 	}{
// 		{"valid", buildFrame(1, true, []byte("CONNECT")), buildFrame(1, false, []byte("GET /foo\r\n\r\n")), false, nil},
// 		{"payload first", buildFrame(1, false, []byte("invalid_cmd")), buildFrame(1, false, []byte("n/a")), true, errors.New("expected command")},
// 		{"two commands", buildFrame(1, true, []byte("CONNECT")), buildFrame(1, true, []byte("double_cmd")), true, errors.New("expected request")},
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

// 	for _, test := range tests {
// 		t.Logf("\ttesting %s", test.name)

// 		buff := new(bytes.Buffer)

// 		if len(test.commandFrame) != 0 {
// 			buff.Grow(len(test.commandFrame))
// 			buff.Write(test.commandFrame)
// 		}

// 		if len(test.payloadFrame) != 0 {
// 			buff.Grow(len(test.payloadFrame))
// 			buff.Write(test.payloadFrame)
// 		}

// 		cmd := s.readCommand(buff)
// 		if test.shouldError {
// 			if cmd.err == nil {
// 				t.Logf("%#v", cmd)
// 				t.Fatal("expected error")
// 			}
// 			if cmd.err.Error() != test.err.Error() {
// 				t.Fatalf("expected (%s) got (%s)", cmd.err, test.err)
// 			}
// 			continue
// 		}

// 		if cmd.err != nil {
// 			t.Fatalf("expected no error, got (%s)", cmd.err)
// 		}

// 		if cmd.channelID != 1 {
// 			t.Fatalf("expected channel 1, got (%d)", cmd.channelID)
// 		}

// 		if !bytes.Contains(test.commandFrame, []byte(cmd.name)) {
// 			t.Fatalf("expected (%v) got (%v)", test.commandFrame, cmd.name)
// 		}

// 		if !bytes.Contains(test.payloadFrame, cmd.request) {
// 			t.Fatalf("expected (%v), got (%v)", test.payloadFrame, cmd.request)
// 		}
// 	}
// 	cancel()
// }

// func TestProcessCommand(t *testing.T) {
// 	t.Log("Testing processCommand")

// 	zerolog.SetGlobalLevel(zerolog.Disabled)

// 	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		fmt.Fprintln(w, "{}")
// 	}))
// 	defer ts.Close()

// 	tests := []struct {
// 		name        string
// 		cmd         command
// 		shouldError bool
// 		err         error
// 	}{
// 		{"valid connect", command{name: "CONNECT", request: []byte("GET /\r\n\r\n")}, false, nil},
// 		{"invalid connect - zero len request", command{name: "CONNECT", request: []byte("")}, true, errors.New("invalid connect command, 0 length request")},
// 		{"valid reset", command{name: "RESET", reset: true}, true, errors.New("received RESET command from broker")},
// 		{"cmd err - ignored (SHUTDOWN)", command{name: "SHUTDOWN", ignore: true}, true, errors.New("unused/empty command (SHUTDOWN)")},
// 		{"cmd err - ignored (empty)", command{name: "", ignore: true}, true, errors.New("unused/empty command ()")},
// 		{"cmd err", command{err: errors.New("command error")}, true, errors.New("command error")},
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
// 	s.agentAddress = ts.Listener.Addr().String()

// 	for _, test := range tests {
// 		t.Logf("\ttesting %s", test.name)

// 		cmd := s.processCommand(test.cmd)
// 		if test.shouldError {
// 			if cmd.err == nil {
// 				t.Logf("%#v", cmd)
// 				t.Fatal("expected error")
// 			}
// 			if cmd.err.Error() != test.err.Error() {
// 				t.Fatalf("expected (%s) got (%s)", test.err, cmd.err)
// 			}
// 			continue
// 		}

// 		if cmd.err != nil {
// 			t.Fatalf("expected no error, got (%s)", cmd.err)
// 		}

// 		if cmd.name == s.cmdConnect {
// 			if cmd.metrics == nil {
// 				t.Fatal("expected metrics to be not nil")
// 			}
// 		}

// 		if cmd.name == s.cmdReset {
// 			if !cmd.reset {
// 				t.Fatal("expected 'reset' to be true")
// 			}
// 		}
// 	}
// 	cancel()
// }
