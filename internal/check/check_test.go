// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"reflect"
	"testing"

	"github.com/circonus-labs/circonus-agent/internal/check/bundle"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

//
// start actual tests for methods in main
//

func TestNew(t *testing.T) {
	t.Log("Testing New")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("check not needed")
	{
		viper.Reset()
		viper.Set(config.KeyCheckBundleID, "")
		viper.Set(config.KeyCheckCreate, false)
		viper.Set(config.KeyReverse, false)
		viper.Set(config.KeyAPITokenKey, "")
		viper.Set(config.KeyAPITokenApp, "")
		viper.Set(config.KeyAPIURL, "")

		_, err := New(nil)
		if err != nil {
			t.Fatalf("expected NO error, got (%s)", err)
		}
	}
}

func TestCheck_CheckMeta(t *testing.T) {
	type fields struct {
		checkBundle *bundle.Bundle
	}
	tests := []struct {
		name    string
		fields  fields
		want    *Meta
		wantErr bool
	}{
		{"nil checkbundle", fields{}, nil, true},
		{"checkbundle (nil bundle)", fields{checkBundle: &bundle.Bundle{}}, nil, true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			c := &Check{
				checkBundle: tt.fields.checkBundle,
			}
			got, err := c.CheckMeta()
			if (err != nil) != tt.wantErr {
				t.Errorf("Check.CheckMeta() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Check.CheckMeta() = %v, want %v", got, tt.want)
			}
		})
	}
}
