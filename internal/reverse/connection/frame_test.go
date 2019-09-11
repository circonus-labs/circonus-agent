// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package connection

// import (
// 	"bytes"
// 	"context"
// 	"errors"
// 	"testing"

// 	"github.com/circonus-labs/circonus-agent/internal/check"
// 	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
// 	"github.com/rs/zerolog"
// )

// func TestBuildFrame(t *testing.T) {
// 	t.Log("Testing buildFrame")

// 	tt := []struct {
// 		description string
// 		expect      []byte
// 		channelID   uint16
// 		command     bool
// 		payload     string
// 	}{
// 		{
// 			description: "command payload",
// 			expect:      []byte{0x80, 0x1, 0x0, 0x0, 0x0, 0x5, 0x52, 0x45, 0x53, 0x45, 0x54},
// 			channelID:   1,
// 			command:     true,
// 			payload:     "RESET",
// 		},
// 		{
// 			description: "data payload",
// 			expect:      []byte{0x0, 0x1, 0x0, 0x0, 0x0, 0xb, 0x7b, 0x22, 0x74, 0x65, 0x73, 0x74, 0x22, 0x3a, 0x20, 0x31, 0x7d},
// 			channelID:   1,
// 			command:     false,
// 			payload:     `{"test": 1}`,
// 		},
// 	}

// 	for _, tst := range tt {
// 		t.Log(tst.description)
// 		data := buildFrame(tst.channelID, tst.command, []byte(tst.payload))
// 		if data == nil {
// 			t.Fatal("expected not nil")
// 		}

// 		if !bytes.Equal(data, tst.expect) {
// 			t.Fatalf("expected (%#v) got (%#v)", tst.expect, data)
// 		}
// 	}
// }

// func TestReadFrameHeader(t *testing.T) {
// 	t.Log("Testing readFrameHeader")

// 	zerolog.SetGlobalLevel(zerolog.Disabled)

// 	tt := []struct {
// 		description string
// 		expect      []byte
// 		channelID   uint16
// 		command     bool
// 		payload     string
// 	}{
// 		{
// 			description: "command header",
// 			channelID:   1,
// 			command:     true,
// 			payload:     "RESET",
// 		},
// 		{
// 			description: "data header",
// 			channelID:   1,
// 			command:     false,
// 			payload:     `{"test": 1}`,
// 		},
// 	}

// 	for _, tst := range tt {
// 		t.Log(tst.description)
// 		data := buildFrame(tst.channelID, tst.command, []byte(tst.payload))
// 		b := bytes.NewReader(data)
// 		hdr, err := readFrameHeader(b)
// 		if err != nil {
// 			t.Fatalf("expected no error, got (%s)", err)
// 		}
// 		if hdr == nil {
// 			t.Fatal("expected packet")
// 		}
// 		if hdr.channelID != tst.channelID {
// 			t.Fatalf("expected channel %d, got (%d)", tst.channelID, hdr.channelID)
// 		}
// 		if hdr.isCommand != tst.command {
// 			t.Fatalf("expected %v, got %v", tst.command, hdr.isCommand)
// 		}
// 		if hdr.payloadLen != uint32(len(tst.payload)) {
// 			t.Fatalf("expected payload length %d, got %d", len(tst.payload), hdr.payloadLen)
// 		}
// 	}
// }

// func TestReadFramePayload(t *testing.T) {
// 	t.Log("Testing readFramePayload")

// 	zerolog.SetGlobalLevel(zerolog.Disabled)

// 	t.Log("zero length")
// 	{
// 		expect := []byte("")
// 		b := bytes.NewReader(expect)
// 		data, err := readFramePayload(b, uint32(b.Len()))
// 		if err != nil {
// 			t.Fatalf("expected no error, got (%s)", err)
// 		}
// 		if string(data) != string(expect) {
// 			t.Fatalf("expected (%s) got (%s)", string(expect), string(data))
// 		}
// 	}

// 	t.Log("undersize")
// 	{
// 		expect := []byte("test")
// 		expectErr := errors.New("invalid read, expected bytes 6 got 4 ([]byte{0x74, 0x65, 0x73, 0x74} = test)")
// 		b := bytes.NewReader(expect)
// 		data, err := readFramePayload(b, 6)
// 		if err == nil {
// 			t.Fatal("expected err")
// 		}
// 		if err.Error() != expectErr.Error() {
// 			t.Fatalf("expected (%s) got (%s)", expectErr, err)
// 		}
// 		if data != nil {
// 			t.Fatalf("expected nil, got %#v %s", data, string(data))
// 		}
// 	}

// 	t.Log("simple")
// 	{
// 		expect := []byte("test")
// 		b := bytes.NewReader(expect)
// 		data, err := readFramePayload(b, uint32(b.Len()))
// 		if err != nil {
// 			t.Fatalf("expected no error, got (%s)", err)
// 		}
// 		if string(data) != string(expect) {
// 			t.Fatalf("expected (%s) got (%s)", string(expect), string(data))
// 		}
// 	}
// }

// func TestReadBytes(t *testing.T) {
// 	t.Log("Testing readBytes")

// 	zerolog.SetGlobalLevel(zerolog.Disabled)

// 	expect := []byte("test")
// 	b := bytes.NewReader(expect)
// 	data, err := readBytes(b, int64(b.Len()))
// 	if err != nil {
// 		t.Fatalf("expected no error, got (%s)", err)
// 	}
// 	if string(data) != string(expect) {
// 		t.Fatalf("expected (%s) got (%s)", string(expect), string(data))
// 	}
// }

// func TestReadFrameFromBroker(t *testing.T) {
// 	t.Log("Testing readFrameFromBroker")

// 	zerolog.SetGlobalLevel(zerolog.Disabled)

// 	tt := []struct {
// 		description string
// 		expect      []byte
// 		channelID   uint16
// 		command     bool
// 		payload     string
// 	}{
// 		{
// 			description: "command payload",
// 			channelID:   1,
// 			command:     true,
// 			payload:     "RESET",
// 		},
// 		{
// 			description: "data payload",
// 			channelID:   1,
// 			command:     false,
// 			payload:     `{"test": 1}`,
// 		},
// 	}

// 	for _, tst := range tt {
// 		t.Log(tst.description)
// 		chk, cerr := check.New(nil)
// 		if cerr != nil {
// 			t.Fatalf("expected no error, got (%s)", cerr)
// 		}
// 		ctx, cancel := context.WithCancel(context.Background())
// 		s, err := New(ctx, chk, defaults.Listen)
// 		if err != nil {
// 			t.Fatalf("expected no error, got (%s)", err)
// 		}
// 		data := buildFrame(tst.channelID, tst.command, []byte(tst.payload))
// 		b := bytes.NewReader(data)
// 		p, err := s.readFrameFromBroker(b)
// 		if err != nil {
// 			t.Fatalf("expected no error, got (%s)", err)
// 		}
// 		if p == nil {
// 			t.Fatal("expected packet")
// 		}
// 		if p.header.channelID != tst.channelID {
// 			t.Fatalf("expected channel %d, got (%d)", tst.channelID, p.header.channelID)
// 		}
// 		if p.header.isCommand != tst.command {
// 			t.Fatalf("expected %v, got %v", tst.command, p.header.isCommand)
// 		}
// 		if p.header.payloadLen != uint32(len(tst.payload)) {
// 			t.Fatalf("expected payload length %d, got %d", len(tst.payload), p.header.payloadLen)
// 		}
// 		if string(p.payload) != tst.payload {
// 			t.Fatalf("expected (%s) got (%s)", tst.payload, string(p.payload))
// 		}
// 		cancel()
// 	}
// }
