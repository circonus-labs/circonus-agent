// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package tags

import (
	"regexp"
	"sort"
	"strings"

	"github.com/circonus-labs/circonus-agent/internal/config"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// Tag aliases cgm's Tag to centralize definition
type Tag = cgm.Tag

// Tags aliases cgm's Tags to centralize definition
type Tags = cgm.Tags

// TaggedMetric definefs a tagged metric
type TaggedMetric struct {
	Tags  *Tags       `json:"_tags"`
	Type  string      `json:"_type"`
	Value interface{} `json:"_value"`
}

// TaggedMetrics is a list of metrics with tags
type TaggedMetrics map[string]TaggedMetric

// JSONMetric defines an individual metric received in JSON
type JSONMetric struct {
	Tags  []string    `json:"_tags"`
	Type  string      `json:"_type"`
	Value interface{} `json:"_value"`
}

// JSONMetrics holds list of JSON metrics received at /write receiver interface
type JSONMetrics map[string]JSONMetric

const (
	// Delimiter defines character separating category from value in a tag e.g. location:london
	Delimiter = ":"
	// Separator defines character separating tags in a list e.g. os:centos,location:sfo
	Separator       = ","
	replacementChar = "_"
)

var (
	valid    = regexp.MustCompile(`^[^:,]+:[^:,]+(,[^:,]+:[^:,]+)*$`)
	cleaner  = regexp.MustCompile(`[\[\]'"` + "`]")
	baseTags *[]string
)

// GetBaseTags returns the check.tags as a list if check.metric_streamtags is true
// ensuring that all metrics have, at a minimum, the same base set of tags
func GetBaseTags() []string {
	if baseTags != nil {
		return *baseTags
	}

	baseTags = &[]string{}

	if !viper.GetBool(config.KeyCheckMetricStreamtags) {
		return *baseTags
	}

	// check.tags is a comma separated list of key:value pairs
	// a) backwards support for how tags were specified in NAD and the JS version of cosi
	// b) viper (at the moment) handles stringSlices different between command line and environment (sigh)
	//    command line takes comma separated list, environment only takes space separated list (tags can contain spaces...)
	tagSpec := viper.GetString(config.KeyCheckTags)
	if tagSpec == "" {
		return *baseTags
	}

	// if systemd ExecStart=circonus-agentd --check-tags="c1:v1,c2:v1" syntax is
	// used, tagSpec will literally be `"c1:v1,c2:v1"` with the quotes included
	// resulting in the first tag having a leading '"' and the last tag having
	// a trailing '"'...
	if tagSpec[0:1] == `"` {
		tagSpec = tagSpec[1:]
		if tagSpec[len(tagSpec)-1:1] == `"` {
			tagSpec = tagSpec[0 : len(tagSpec)-1]
		}
	}

	checkTags := strings.Split(tagSpec, Separator)
	if len(checkTags) == 0 {
		return *baseTags
	}

	tags := make([]string, 0, len(checkTags))
	tags = append(tags, checkTags...)
	baseTags = &tags

	return *baseTags
}

// FromString convert old style tag string spec "cat:val,cat:val,..." into a Tags structure
func FromString(tags string) Tags {
	if tags == "" || !valid.MatchString(tags) {
		return Tags{}
	}
	tagList := strings.Split(tags, Separator)
	return FromList(tagList)
}

// FromList convert old style list of tags []string{"cat:val","cat:val",...} into a Tags structure
func FromList(tagList []string) Tags {
	if len(tagList) == 0 {
		return Tags{}
	}

	tags := make(Tags, 0, len(tagList))
	for _, tag := range tagList {
		t := strings.SplitN(tag, Delimiter, 2)
		if len(t) != 2 {
			continue // must be *only* two
		}
		tags = append(tags, Tag{t[0], t[1]})
	}

	return tags
}

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
