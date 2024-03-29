// Copyright © 2020 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

//go:build !openbsd
// +build !openbsd

package bundle

import (
	"strings"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/spf13/viper"
)

func (cb *Bundle) getHostTags() []string {
	var tags []string

	chkTags := viper.GetString(config.KeyCheckTags)
	if chkTags != "" {
		ctags := strings.Split(chkTags, ",")
		for tpos, tag := range ctags {
			parts := strings.SplitN(tag, ":", 2)
			if parts == nil {
				cb.logger.Warn().Int("pos", tpos).Str("tag", tag).Msg("invalid check tag")
				continue
			}
			ntag := strings.ToLower(strings.TrimSpace(parts[0]))
			ntag += ":"
			if len(parts) == 2 {
				ntag += parts[1]
			}
			tags = append(tags, ntag)
		}
	}

	hi, err := host.Info()
	if err != nil {
		cb.logger.Warn().Err(err).Msg("unable to get host info for check tags")
		return tags
	}

	if hi.OS != "" {
		tags = append(tags, "os:"+hi.OS)
	}
	if hi.Platform != "" {
		tags = append(tags, "platform:"+hi.Platform)
	}
	if hi.PlatformFamily != "" {
		tags = append(tags, "platform_family:"+hi.PlatformFamily)
	}
	if hi.PlatformVersion != "" {
		tags = append(tags, "platform_version:"+hi.PlatformVersion)
	}
	if hi.KernelVersion != "" {
		tags = append(tags, "kernel_version:"+hi.KernelVersion)
	}
	if hi.KernelArch != "" {
		tags = append(tags, "kernel_arch:"+hi.KernelArch)
	}
	if hi.VirtualizationSystem != "" {
		tags = append(tags, "virt_sys:"+hi.VirtualizationSystem)
	}
	if hi.VirtualizationRole != "" {
		tags = append(tags, "virt_role:"+hi.VirtualizationRole)
	}

	return tags
}
