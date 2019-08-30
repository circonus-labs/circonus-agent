// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package bundle

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/circonus-labs/go-apiclient"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestBundle_isValidBroker(t *testing.T) {

	zerolog.SetGlobalLevel(zerolog.Disabled)

	type fields struct {
		statusActiveBroker    string
		brokerMaxResponseTime time.Duration
		brokerMaxRetries      int
		logger                zerolog.Logger
	}
	type args struct {
		broker    *apiclient.Broker
		checkType string
	}

	defaultFields := fields{
		statusActiveBroker:    "active",
		brokerMaxResponseTime: time.Millisecond * 5,
		brokerMaxRetries:      2,
		logger:                log.With().Logger(),
	}

	tests := []struct {
		name           string
		fields         fields
		args           args
		want           time.Duration
		want1          bool
		needTestServer bool
	}{
		{"bad broker (nil)", defaultFields, args{broker: nil, checkType: ""}, 0, false, false},
		{"bad check type (empty)", defaultFields, args{broker: &testBroker, checkType: ""}, 0, false, false},
		{"can't reach", defaultFields, args{broker: &testBroker, checkType: "json"}, 0, false, false},
		{"valid", defaultFields, args{broker: &testBroker, checkType: "json"}, time.Millisecond * 100, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := &Bundle{
				statusActiveBroker:    tt.fields.statusActiveBroker,
				brokerMaxResponseTime: tt.fields.brokerMaxResponseTime,
				brokerMaxRetries:      tt.fields.brokerMaxRetries,
				logger:                tt.fields.logger,
			}
			var ts *httptest.Server
			if tt.needTestServer {
				ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprintln(w, "ok")
				}))
				u, err := url.Parse(ts.URL)
				if err != nil {
					t.Fatalf("unable to start test server (%s)", err)
				}
				host := u.Hostname()
				p, e := strconv.Atoi(u.Port())
				if e != nil {
					t.Fatalf("unable to convert test server port to int (%s)", e)
				}
				port := uint16(p)
				tt.args.broker = &apiclient.Broker{
					CID:  "/broker/1234",
					Type: "enterprise",
					Details: []apiclient.BrokerDetail{
						apiclient.BrokerDetail{
							IP:      &host,
							Port:    &port,
							Modules: []string{"json"},
							Status:  StatusActive,
						},
					},
				}
			}
			got, got1 := cb.isValidBroker(tt.args.broker, tt.args.checkType)
			if tt.needTestServer {
				ts.Close()
			}
			if got > tt.want {
				t.Errorf("Bundle.isValidBroker() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Bundle.isValidBroker() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_brokerSupportsCheckType(t *testing.T) {
	defaultDetails := &apiclient.BrokerDetail{Modules: []string{"json", "httptrap"}}
	type args struct {
		checkType string
		details   *apiclient.BrokerDetail
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"no type", args{checkType: "", details: nil}, false},
		{"nil details", args{checkType: "json", details: nil}, false},
		{"unsupported type", args{checkType: "invalid", details: defaultDetails}, false},
		{"supported type (json)", args{checkType: "json", details: defaultDetails}, true},
		{"supported subtype (json:nad)", args{checkType: "json:nad", details: defaultDetails}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := brokerSupportsCheckType(tt.args.checkType, tt.args.details); got != tt.want {
				t.Errorf("brokerSupportsCheckType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBundle_selectBroker(t *testing.T) {

	zerolog.SetGlobalLevel(zerolog.Disabled)

	host := "127.0.0.1"
	port := uint16(123)
	noValidBrokers := []apiclient.Broker{
		apiclient.Broker{
			CID:  "/broker/123",
			Type: "enterprise",
			Details: []apiclient.BrokerDetail{
				apiclient.BrokerDetail{
					IP:      &host,
					Port:    &port,
					Modules: []string{"json"},
					Status:  StatusActive,
				},
			},
		},
	}

	type fields struct {
		statusActiveBroker    string
		brokerMaxResponseTime time.Duration
		brokerMaxRetries      int
		logger                zerolog.Logger
	}

	defaultFields := fields{
		statusActiveBroker:    "active",
		brokerMaxResponseTime: time.Millisecond * 100,
		brokerMaxRetries:      2,
		logger:                log.With().Logger(),
	}

	type args struct {
		checkType  string
		brokerList *[]apiclient.Broker
	}
	tests := []struct {
		name           string
		fields         fields
		args           args
		want           *apiclient.Broker
		wantErr        bool
		needTestServer bool
	}{
		{"invalid check type (empty)", defaultFields, args{checkType: "", brokerList: nil}, nil, true, false},
		{"invalid broker list (nil)", defaultFields, args{checkType: "json:nad", brokerList: nil}, nil, true, false},
		{"invalid broker list (empty)", defaultFields, args{checkType: "json:nad", brokerList: &[]apiclient.Broker{}}, nil, true, false},
		{"no valid broker", defaultFields, args{checkType: "json:nad", brokerList: &noValidBrokers}, nil, true, false},
		{"valid", defaultFields, args{checkType: "json:nad", brokerList: &noValidBrokers}, nil, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := &Bundle{
				statusActiveBroker:    tt.fields.statusActiveBroker,
				brokerMaxResponseTime: tt.fields.brokerMaxResponseTime,
				brokerMaxRetries:      tt.fields.brokerMaxRetries,
				logger:                tt.fields.logger,
			}
			// bl := tt.args.brokerList
			var ts *httptest.Server
			if tt.needTestServer {
				ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprintln(w, "ok")
				}))
				u, err := url.Parse(ts.URL)
				if err != nil {
					t.Fatalf("unable to start test server (%s)", err)
				}
				host := u.Hostname()
				p, e := strconv.Atoi(u.Port())
				if e != nil {
					t.Fatalf("unable to convert test server port to int (%s)", e)
				}
				port := uint16(p)
				// test broker list argument with the dynamically created broker details
				tt.args.brokerList = &[]apiclient.Broker{
					apiclient.Broker{
						CID:  "/broker/4321",
						Type: "enterprise",
						Details: []apiclient.BrokerDetail{
							apiclient.BrokerDetail{
								IP:      &host,
								Port:    &port,
								Modules: []string{"json"},
								Status:  StatusActive,
							},
						},
					},
				}
				// set the expected result to the dynamically created broker
				tt.want = &(*tt.args.brokerList)[0]
			}
			got, err := cb.selectBroker(tt.args.checkType, tt.args.brokerList)
			if tt.needTestServer {
				ts.Close()
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("Bundle.selectBroker() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Bundle.selectBroker() = %v, want %v", got, tt.want)
			}
		})
	}
}
