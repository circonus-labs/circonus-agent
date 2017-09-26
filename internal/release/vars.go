// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package release

const (
	// NAME is the name of this application
	NAME = "circonus-agent"
	// ENVPREFIX is the environment variable prefix
	ENVPREFIX = "NAD"
)

// vars are manipulated at link time (see goreleaser)
var (
	// COMMIT of relase in git repo
	COMMIT = "none"
	// DATE of release
	DATE = "unknown"
	// TAG of release
	TAG = ""
	// VERSION of the release
	VERSION = "dev"
)
