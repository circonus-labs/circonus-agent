// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package tags

import "regexp"

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
