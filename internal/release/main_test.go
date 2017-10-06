// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package release

import "testing"

func TestInfo(t *testing.T) {
	t.Log("Testing info")

	x := info()
	if x == nil {
		t.Fatal("expected not nil")
	}
}
