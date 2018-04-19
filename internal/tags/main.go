// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package tags

import (
	"sort"
	"strings"

	"github.com/pkg/errors"
)

// PrepStreamTags accepts a comma delimited list of key:value pairs
// and returns a stream tag formatted spec or an error if there are
// issues with the format of the supplied tag list.
func PrepStreamTags(tagList string) (string, error) {
	if tagList == "" {
		return "", nil
	}

	if !valid.MatchString(tagList) {
		return "", errors.Errorf("invalid tag format")
	}

	t := strings.Split(cleaner.ReplaceAllString(tagList, replacementChar), Separator)

	// so that components which treat metric names as simple strings
	// receive a consistent, predictive metric name
	sort.Strings(t)

	return "|ST[" + strings.Join(t, Separator) + "]", nil
}
