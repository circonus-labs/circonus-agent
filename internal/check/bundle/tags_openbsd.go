// Copyright © 2020 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build openbsd

package bundle

import (
	"runtime"
	"strings"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/spf13/viper"
)

func (cb *Bundle) getHostTags() []string {
	var tags []string

	chkTags := viper.GetString(config.KeyCheckTags)
	if chkTags != "" {
		tags = append(tags, strings.Split(chkTags, ",")...)
	}

	// gopsutil does not compile on openbsd at the moment, use basic info for tags
	tags = append(tags, []string{"os:" + runtime.GOOS, "kernel_arch:" + runtime.GOARCH}...)

	return tags
}
