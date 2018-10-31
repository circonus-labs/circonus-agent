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
