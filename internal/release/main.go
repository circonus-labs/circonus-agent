// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package release

import (
	"expvar"
)

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
