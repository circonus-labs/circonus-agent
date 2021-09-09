// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

//go:build go1.17
// +build go1.17

package main

import (
	"github.com/circonus-labs/circonus-agent/cmd"
)

func main() {
	cmd.Execute()
}
