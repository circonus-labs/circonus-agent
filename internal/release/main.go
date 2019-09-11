// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package release

import (
	"expvar"
)

const (
	// NAME is the name of this application
	NAME = "circonus-agent"
	// ENVPREFIX is the environment variable prefix
	ENVPREFIX = "CA"
)

// defined during build (e.g. goreleaser, see .goreleaser.yml)
var (
	// COMMIT of relase in git repo
	COMMIT = "undef"
	// DATE of release
	DATE = "undef"
	// TAG of release
	TAG = "none"
	// VERSION of the release
	VERSION = "Dev"
)

// Info contains release information
type Info struct {
	Name      string
	Version   string
	Commit    string
	BuildDate string
	Tag       string
}

func init() {
	expvar.Publish("app", expvar.Func(info))
}

func info() interface{} {
	return &Info{
		Name:      NAME,
		Version:   VERSION,
		Commit:    COMMIT,
		BuildDate: DATE,
		Tag:       TAG,
	}
}
