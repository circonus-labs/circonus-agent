// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package tags

import (
	"regexp"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

// JSONMetric defines an individual metric received in JSON
type JSONMetric struct {
	Tags  []string    `json:"_tags"`
	Type  string      `json:"_type"`
	Value interface{} `json:"_value"`
}

// JSONMetrics holds list of JSON metrics
type JSONMetrics map[string]JSONMetric

const (
	// Delimiter defines character separating category from value in a tag e.g. location:london
	Delimiter = ":"
	// Separator defines character separating tags in a list e.g. os:centos,location:sfo
	Separator       = ","
	replacementChar = "_"
)

var (
	valid   = regexp.MustCompile(`^[^:,]+:[^:,]+(,[^:,]+:[^:,]+)*$`)
	cleaner = regexp.MustCompile(`[\[\]'"` + "`]")
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
