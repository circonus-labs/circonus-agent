// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package bundle

import (
	"errors"
	"reflect"
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/circonus-labs/go-apiclient"
	"github.com/gojuno/minimock/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func TestNew(t *testing.T) {
	type args struct {
		client API
	}
	tests := []struct {
		args    args
		want    *Bundle
		name    string
		wantErr bool
	}{
		{name: "invalid (nil client)", args: args{client: nil}, want: nil, wantErr: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.args.client)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFetchCheck(t *testing.T) {
	t.Log("Testing fetchCheck")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	viper.Reset()
	viper.Set(config.KeyAPITokenKey, "foo")
	viper.Set(config.KeyAPITokenApp, "bar")
	viper.Set(config.KeyAPIURL, "baz")

	mc := minimock.NewController(t)
	client := genMockClient(mc)
	c := Bundle{
		client: client,
	}

	t.Log("cid (empty)")
	{
		cid := ""
		viper.Set(config.KeyCheckBundleID, cid)

		_, err := c.fetchCheckBundle(cid)
		if err == nil {
			t.Fatal("expected error")
		}

		if !errors.Is(err, errInvalidCID) {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}

	t.Log("cid (abc)")
	{
		cid := "abc"
		viper.Set(config.KeyCheckBundleID, cid)

		_, err := c.fetchCheckBundle(cid)
		if err == nil {
			t.Fatal("expected error")
		}

		if !errors.Is(err, errInvalidCID) {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}

	t.Log("api error")
	{
		cid := "000"
		viper.Set(config.KeyCheckBundleID, cid)

		_, err := c.fetchCheckBundle(cid)
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != "unable to retrieve check bundle (/check_bundle/000): forced mock api call error" {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}

	t.Log("valid")
	{
		cid := "1234"
		viper.Set(config.KeyCheckBundleID, cid)

		_, err := c.fetchCheckBundle(cid)
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
	}
}

func TestFindCheck(t *testing.T) {
	t.Log("Testing findCheck")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	viper.Reset()
	viper.Set(config.KeyAPITokenKey, "foo")
	viper.Set(config.KeyAPITokenApp, "bar")
	viper.Set(config.KeyAPIURL, "baz")

	mc := minimock.NewController(t)
	client := genMockClient(mc)
	c := Bundle{client: client}

	t.Log("target (empty)")
	{

		target := ""
		viper.Set(config.KeyCheckTarget, target)

		_, found, err := c.findCheckBundle()
		if err == nil {
			t.Fatal("expected error")
		}
		if found != -1 {
			t.Fatal("expected found == -1")
		}

		if err.Error() != "invalid check bundle target (empty)" {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}

	t.Log("api error")
	{
		target := "000"
		viper.Set(config.KeyCheckTarget, target)

		_, found, err := c.findCheckBundle()
		if err == nil {
			t.Fatal("expected error")
		}
		if found != -1 {
			t.Fatal("expected found == -1")
		}

		if err.Error() != "searching for check bundle: forced mock api call error" {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}

	t.Log("not found")
	{
		target := "not_found"
		viper.Set(config.KeyCheckTarget, target)

		_, found, err := c.findCheckBundle()
		if err == nil {
			t.Fatal("expected error")
		}
		if found != 0 {
			t.Fatal("expected found == 0")
		}

		if err.Error() != `criteria - (active:1)(type:"`+defaults.CheckType+`")(target:"not_found"): no check bundles matched` {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}

	t.Log("multiple w/2 agent check")
	{
		target := "multiple2"
		viper.Set(config.KeyCheckTarget, target)

		_, found, err := c.findCheckBundle()
		if err == nil {
			t.Fatal("expected error")
		}
		if found != 2 {
			t.Fatal("expected found == 2")
		}

		if err.Error() != `multiple check bundles (2) found matching criteria ((active:1)(type:"`+defaults.CheckType+`")(target:"multiple2")) created by (circonus-agent)` {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}

	t.Log("multiple w/1 agent check")
	{
		target := "multiple1"
		viper.Set(config.KeyCheckTarget, target)

		_, found, err := c.findCheckBundle()
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
		if found != 1 {
			t.Fatal("expected found == 1")
		}
	}

	t.Log("multiple w/0 agent check")
	{
		target := "multiple0"
		viper.Set(config.KeyCheckTarget, target)

		_, found, err := c.findCheckBundle()
		if err == nil {
			t.Fatal("expected error")
		}
		if found != 0 {
			t.Fatal("expected found == 0")
		}

		if err.Error() != `multiple check bundles (2) found matching criteria ((active:1)(type:"`+defaults.CheckType+`")(target:"multiple0")), none created by (circonus-agent)` {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}

	t.Log("valid")
	{
		target := "valid"
		viper.Set(config.KeyCheckTarget, target)

		_, found, err := c.findCheckBundle()
		if err != nil {
			t.Fatalf("expected no error, got (%s)", err)
		}
		if found != 1 {
			t.Fatal("expected found == 1")
		}
	}
}

func TestCreateCheck(t *testing.T) {
	t.Log("Testing createCheck")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	viper.Reset()
	viper.Set(config.KeyAPITokenKey, "foo")
	viper.Set(config.KeyAPITokenApp, "bar")
	viper.Set(config.KeyAPIURL, "baz")

	mc := minimock.NewController(t)
	client := genMockClient(mc)
	c := Bundle{client: client}

	t.Log("target (empty)")
	{
		target := ""
		viper.Set(config.KeyCheckTarget, target)

		_, err := c.createCheckBundle()
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != "invalid check bundle target (empty)" {
			t.Fatalf("unexpected error return (%s)", err)
		}
	}
}

func TestBundle_CID(t *testing.T) {
	type fields struct {
		bundle *apiclient.CheckBundle
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
		{"nil bundle", fields{}, "", true},
		{"valid bundle", fields{bundle: &testCheckBundle}, "/check_bundle/1234", false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cb := &Bundle{
				bundle: tt.fields.bundle,
			}
			got, err := cb.CID()
			if (err != nil) != tt.wantErr {
				t.Errorf("Bundle.CID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Bundle.CID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBundle_Period(t *testing.T) {
	type fields struct {
		bundle *apiclient.CheckBundle
	}
	tests := []struct {
		fields  fields
		name    string
		want    uint
		wantErr bool
	}{
		{name: "nil bundle", fields: fields{}, want: 0, wantErr: true},
		{name: "valid bundle", fields: fields{bundle: &testCheckBundle}, want: 60, wantErr: false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cb := &Bundle{
				bundle: tt.fields.bundle,
			}
			got, err := cb.Period()
			if (err != nil) != tt.wantErr {
				t.Errorf("Bundle.Period() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Bundle.Period() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBundle_Refresh(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	viper.Set(config.KeyCheckBundleID, "/check_bundle/1234")
	mc := minimock.NewController(t)
	client := genMockClient(mc)
	tb := testCheckBundle
	type fields struct {
		bundle *apiclient.CheckBundle
		client API
		logger zerolog.Logger
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{"nil bundle", fields{bundle: nil, client: client, logger: log.With().Logger()}, true},
		{"valid", fields{bundle: &tb, client: client, logger: log.With().Logger()}, false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cb := &Bundle{
				bundle: tt.fields.bundle,
				client: tt.fields.client,
				logger: tt.fields.logger,
			}
			if err := cb.Refresh(); (err != nil) != tt.wantErr {
				t.Errorf("Bundle.Refresh() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBundle_CheckCID(t *testing.T) {
	tb := testCheckBundle
	type fields struct {
		bundle *apiclient.CheckBundle
	}
	type args struct {
		idx uint
	}
	tests := []struct {
		fields  fields
		name    string
		want    string
		args    args
		wantErr bool
	}{
		{name: "invalid (nil bundle)", fields: fields{bundle: nil}, args: args{idx: 0}, want: "", wantErr: true},
		{name: "invalid (no checks in bundle)", fields: fields{bundle: &apiclient.CheckBundle{}}, args: args{idx: 0}, want: "", wantErr: true},
		{name: "invalid (idx out of range)", fields: fields{bundle: &tb}, args: args{idx: 10}, want: "", wantErr: true},
		{name: "valid", fields: fields{bundle: &tb}, args: args{idx: 0}, want: testCheckBundle.Checks[0], wantErr: false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cb := &Bundle{
				bundle: tt.fields.bundle,
			}
			got, err := cb.CheckCID(tt.args.idx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Bundle.CheckCID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Bundle.CheckCID() = %v, want %v", got, tt.want)
			}
		})
	}
}
