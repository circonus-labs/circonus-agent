// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"testing"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/check/bundle"
	"github.com/circonus-labs/go-apiclient"
	"github.com/rs/zerolog"
)

func TestCheck_setReverseConfigs(t *testing.T) {
	type fields struct {
		statusActiveBroker    string
		brokerMaxResponseTime time.Duration
		brokerMaxRetries      int
		checkConfig           *apiclient.Check
		checkBundle           *bundle.Bundle
		broker                *apiclient.Broker
		client                API
		logger                zerolog.Logger
		refreshTTL            time.Duration
		reverse               bool
		revConfigs            *ReverseConfigs
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			c := &Check{
				statusActiveBroker:    tt.fields.statusActiveBroker,
				brokerMaxResponseTime: tt.fields.brokerMaxResponseTime,
				brokerMaxRetries:      tt.fields.brokerMaxRetries,
				checkConfig:           tt.fields.checkConfig,
				checkBundle:           tt.fields.checkBundle,
				broker:                tt.fields.broker,
				client:                tt.fields.client,
				logger:                tt.fields.logger,
				refreshTTL:            tt.fields.refreshTTL,
				reverse:               tt.fields.reverse,
				revConfigs:            tt.fields.revConfigs,
			}
			if err := c.setReverseConfigs(); (err != nil) != tt.wantErr {
				t.Errorf("Check.setReverseConfigs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheck_FindPrimaryBrokerInstance(t *testing.T) {
	type fields struct {
		statusActiveBroker    string
		brokerMaxResponseTime time.Duration
		brokerMaxRetries      int
		checkConfig           *apiclient.Check
		checkBundle           *bundle.Bundle
		broker                *apiclient.Broker
		client                API
		logger                zerolog.Logger
		refreshTTL            time.Duration
		reverse               bool
		revConfigs            *ReverseConfigs
	}
	type args struct {
		cfgs *ReverseConfigs
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			c := &Check{
				statusActiveBroker:    tt.fields.statusActiveBroker,
				brokerMaxResponseTime: tt.fields.brokerMaxResponseTime,
				brokerMaxRetries:      tt.fields.brokerMaxRetries,
				checkConfig:           tt.fields.checkConfig,
				checkBundle:           tt.fields.checkBundle,
				broker:                tt.fields.broker,
				client:                tt.fields.client,
				logger:                tt.fields.logger,
				refreshTTL:            tt.fields.refreshTTL,
				reverse:               tt.fields.reverse,
				revConfigs:            tt.fields.revConfigs,
			}
			got, err := c.FindPrimaryBrokerInstance(tt.args.cfgs)
			if (err != nil) != tt.wantErr {
				t.Errorf("Check.FindPrimaryBrokerInstance() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Check.FindPrimaryBrokerInstance() = %v, want %v", got, tt.want)
			}
		})
	}
}
