// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build linux

package builtins

import (
	"github.com/circonus-labs/circonus-agent/internal/builtins/collector/linux/procfs"
)

func (b *Builtins) configure() error {
	collectors, err := procfs.New()
	if err != nil {
		return err
	}
	for _, c := range collectors {
		b.collectors[c.ID()] = c
	}
	return nil
}
