// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build !windows,!linux

package builtins

func (b *Builtins) configure() error {
	return nil
}
