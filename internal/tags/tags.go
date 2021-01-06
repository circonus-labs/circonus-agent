// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package tags

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/circonus-labs/circonus-agent/internal/config"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/rs/zerolog/log"
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

	MAX_TAGS            = 256  //nolint: golint
	MAX_METRIC_NAME_LEN = 4096 //nolint: golint
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
			log.Warn().Int("num", len(t)).Str("tag", tag).Msg("invalid tag format, ignoring")
			continue // must be *only* two
		}
		tags = append(tags, Tag{Category: t[0], Value: t[1]})
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
		return "", fmt.Errorf("invalid tag format")
	}

	t := strings.Split(cleaner.ReplaceAllString(tagList, replacementChar), Separator)

	// so that components which treat metric names as simple strings
	// receive a consistent, predictive metric name
	sort.Strings(t)

	return "|ST[" + strings.Join(t, Separator) + "]", nil
}

// MetricNameWithStreamTags will encode tags as stream tags into supplied metric name.
// Note: if metric name already has stream tags it is assumed the metric name and
// embedded stream tags are being managed manually and calling this method will nave no effect.
func MetricNameWithStreamTags(metric string, tags Tags) string {
	if len(tags) == 0 {
		return metric
	}

	if strings.Contains(metric, "|ST[") {
		return metric
	}

	taglist := EncodeMetricStreamTags(tags)
	if taglist != "" {
		return metric + "|ST[" + taglist + "]"
	}

	return metric
}

func MergeTags(metricName string, metricTags []string) string {
	if len(metricTags) == 0 {
		return metricName
	}
	if !strings.Contains(metricName, "|ST[") {
		return EncodeMetricStreamTags(FromList(metricTags))
	}

	p1 := strings.SplitN(metricName, "|ST[", 2)
	if len(p1) != 2 {
		return EncodeMetricStreamTags(FromList(metricTags))
	}

	mn := p1[0]
	mt := strings.TrimSuffix(p1[1], "]")

	tl := strings.Split(mt, ",")
	tl = append(tl, metricTags...)
	st := EncodeMetricStreamTags(FromList(tl))

	if st != "" {
		return mn + "|ST[" + st + "]"
	}

	return mn
}

// EncodeMetricStreamTags encodes Tags into a string suitable for use with
// stream tags. Tags directly embedded into metric names using the
// `metric_name|ST[<tags>]` syntax.
func EncodeMetricStreamTags(tags Tags) string {
	if len(tags) == 0 {
		return ""
	}

	tmpTags := EncodeMetricTags(tags)
	if len(tmpTags) == 0 {
		return ""
	}

	tagList := make([]string, len(tmpTags))
	encodeFmt := `b"%s"`
	encodedSig := `b"` // has cat or val been previously (or manually) base64 encoded and formatted
	for i, tag := range tmpTags {
		if i >= MAX_TAGS {
			log.Warn().Int("num", len(tags)).Int("max", MAX_TAGS).Interface("tags", tags).Msg("ignoring tags over max")
			break
		}
		tagParts := strings.SplitN(tag, ":", 2)
		if len(tagParts) != 2 {
			log.Warn().Int("num", len(tagParts)).Str("tag", tag).Msg("invalid tag format, ignoring")
			continue // invalid tag, skip it
		}
		tc := tagParts[0]
		tv := tagParts[1]
		if !strings.HasPrefix(tc, encodedSig) {
			tc = fmt.Sprintf(encodeFmt, base64.StdEncoding.EncodeToString([]byte(strings.ToLower(tc))))
		}
		if !strings.HasPrefix(tv, encodedSig) {
			tv = fmt.Sprintf(encodeFmt, base64.StdEncoding.EncodeToString([]byte(tv)))
		}
		tagList[i] = tc + ":" + tv
	}

	return strings.Join(tagList, ",")
}

// EncodeMetricTags encodes Tags into an array of strings. The format
// check_bundle.metircs.metric.tags needs. This helper is intended to work
// with legacy check bundle metrics. Tags directly on named metrics are
// being dropped in favor of stream tags.
func EncodeMetricTags(tags Tags) []string {
	if len(tags) == 0 {
		return []string{}
	}

	uniqueTags := make(map[string]bool)
	encodedSig := `b"` // has cat or val been previously (or manually) base64 encoded and formatted
	for i, t := range tags {
		if i >= MAX_TAGS {
			log.Warn().Int("num", len(tags)).Int("max", MAX_TAGS).Interface("tags", tags).Msg("too many tags, ignoring remainder")
			break
		}
		tc := t.Category
		tv := t.Value
		if !strings.HasPrefix(tc, encodedSig) {
			tc = strings.Map(removeSpaces, strings.ToLower(t.Category))
		}
		if tc == "" || tv == "" {
			log.Warn().Interface("tag", t).Msg("invalid tag format, ignoring")
			continue // invalid tag, skip it
		}
		tag := tc + ":" + tv
		uniqueTags[tag] = true
	}
	tagList := make([]string, len(uniqueTags))
	idx := 0
	for t := range uniqueTags {
		tagList[idx] = t
		idx++
	}
	sort.Strings(tagList)
	return tagList
}

func removeSpaces(r rune) rune {
	if unicode.IsSpace(r) {
		return -1
	}
	return r
}
