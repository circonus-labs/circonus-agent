// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package receiver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/openhistogram/circonusllhist"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

var (
	metricsmu        sync.Mutex
	metrics          *cgm.CirconusMetrics
	baseTags         []string
	histogramRx      *regexp.Regexp // encoded histogram regular express (e.g. coming from a cgm put to /write)
	numFloatRx       *regexp.Regexp // is number or float
	histogramRxNames []string
	logger           = log.With().Str("pkg", "receiver").Logger()
)

func init() {
	histogramRx = regexp.MustCompile(`H\[(?P<bucket>[^\]]+)\]=(?P<count>[0-9]+)`)
	histogramRxNames = histogramRx.SubexpNames()
	numFloatRx = regexp.MustCompile(`^[0-9][0-9]*(\.[0-9]+)?$`)
}

// logshim is used to satisfy apiclient Logger interface (avoiding ptr receiver issue).
type logshim struct {
	logh zerolog.Logger
}

func (l logshim) Printf(fmt string, v ...interface{}) {
	l.logh.Printf(fmt, v...)
}

func initCGM() error {
	metricsmu.Lock()
	defer metricsmu.Unlock()

	if metrics != nil {
		return nil
	}

	cmc := &cgm.Config{
		Debug: viper.GetBool(config.KeyDebugCGM),
		Log:   logshim{logh: logger.With().Str("pkg", "cgm.receiver").Logger()},
	}
	// put cgm into manual mode (no interval, no api key, invalid submission url)
	cmc.Interval = "0"                            // disable automatic flush
	cmc.CheckManager.Check.SubmissionURL = "none" // disable check management (create/update)

	hm, err := cgm.NewCirconusMetrics(cmc)
	if err != nil {
		return fmt.Errorf("receiver cgm: %w", err)
	}

	metrics = hm

	baseTags = tags.GetBaseTags()
	baseTags = append(baseTags, []string{
		"source:" + release.NAME,
		"collector:write",
	}...)

	return nil
}

// Flush returns current metrics.
func Flush() *cgm.Metrics {
	_ = initCGM()
	return metrics.FlushMetrics()
}

// Parse handles incoming PUT/POST requests.
func Parse(id string, data io.Reader) error {
	if err := initCGM(); err != nil {
		return err
	}

	var tmp tags.JSONMetrics // cgm.Metrics
	if err := json.NewDecoder(data).Decode(&tmp); err != nil {
		var serr *json.SyntaxError
		if errors.As(err, &serr) {
			return fmt.Errorf("id:%s - offset %d -- %w", id, serr.Offset, err)
		}
		return fmt.Errorf("parsing json for %s: %w", id, err)
	}

	for name, metric := range tmp {
		metricName := name

		tagList := make([]string, 0, len(baseTags)+len(metric.Tags)+1)
		tagList = append(tagList, baseTags...)
		tagList = append(tagList, metric.Tags...)
		tagList = append(tagList, "collector_id:"+id)
		metricTags := tags.FromList(tagList)

		if strings.Contains(metricName, "|ST") {
			// merge metricTags into the already present streamtags
			metricName = tags.MergeTags(metricName, tagList)
		}

		switch metric.Type {
		case "i":
			if v := parseInt32(metricName, metric); v != nil {
				metrics.AddGaugeWithTags(metricName, metricTags, *v)
			}
		case "I":
			if v := parseUint32(metricName, metric); v != nil {
				metrics.AddGaugeWithTags(metricName, metricTags, *v)
			}
		case "l":
			if v := parseInt64(metricName, metric); v != nil {
				metrics.AddGaugeWithTags(metricName, metricTags, *v)
			}
		case "L":
			if v := parseUint64(metricName, metric); v != nil {
				metrics.AddGaugeWithTags(metricName, metricTags, *v)
			}
		case "h":
			switch metric.Value.(type) { // nolint:gocritic
			case string:
				// convert to []interface{} which is what histogram parser expects
				v := make([]interface{}, 1)
				v[0] = metric.Value.(string)
				metric.Value = v
			default:
			}
			samples := parseHistogram(metricName, metric)
			if samples != nil && len(*samples) > 0 {
				for _, sample := range *samples {
					if sample.bucket {
						metrics.RecordCountForValueWithTags(metricName, metricTags, sample.value, sample.count)
					} else {
						metrics.RecordValueWithTags(metricName, metricTags, sample.value)
					}
				}
			}
		case "n":
			v := parseFloat(metricName, metric)
			if v != nil {
				metrics.AddGaugeWithTags(metricName, metricTags, *v)
			}
		case "s":
			metrics.SetTextWithTags(metricName, metricTags, fmt.Sprintf("%v", metric.Value))
		default:
			logger.Warn().Str("metric", metricName).Str("type", metric.Type).Str("pkg", "receiver").Msg("unsupported metric type")
		}
	}

	return nil
}

func parseInt32(metricName string, metric tags.JSONMetric) *int32 {
	switch t := metric.Value.(type) {
	case float64:
		v := int32(metric.Value.(float64))
		return &v
	case string:
		v, err := strconv.ParseInt(metric.Value.(string), 10, 32)
		if err == nil {
			v2 := int32(v)
			return &v2
		}
		logger.Error().
			Str("metric", metricName).
			Interface("value", metric).
			Err(err).
			Msg("parsing")
	default:
		logger.Error().
			Str("metric", metricName).
			Interface("value", metric).
			Str("type", fmt.Sprintf("%T", t)).
			Msg("invalid value type for metric type")
	}
	return nil
}

func parseUint32(metricName string, metric tags.JSONMetric) *uint32 {
	switch t := metric.Value.(type) {
	case float64:
		v := uint32(metric.Value.(float64))
		return &v
	case string:
		v, err := strconv.ParseUint(metric.Value.(string), 10, 32)
		if err == nil {
			v2 := uint32(v)
			return &v2
		}
		logger.Error().
			Str("metric", metricName).
			Interface("value", metric).
			Err(err).
			Msg("parsing")
	default:
		logger.Error().
			Str("metric", metricName).
			Interface("value", metric).
			Str("type", fmt.Sprintf("%T", t)).
			Msg("invalid value type for metric type")
	}
	return nil
}

func parseInt64(metricName string, metric tags.JSONMetric) *int64 {
	switch t := metric.Value.(type) {
	case float64:
		v := int64(metric.Value.(float64))
		return &v
	case string:
		v, err := strconv.ParseInt(metric.Value.(string), 10, 64)
		if err == nil {
			v2 := v
			return &v2
		}
		logger.Error().
			Str("metric", metricName).
			Interface("value", metric).
			Err(err).
			Msg("parsing")
	default:
		logger.Error().
			Str("metric", metricName).
			Interface("value", metric).
			Str("type", fmt.Sprintf("%T", t)).
			Msg("invalid value type for metric type")
	}
	return nil
}

func parseUint64(metricName string, metric tags.JSONMetric) *uint64 {
	switch t := metric.Value.(type) {
	case float64:
		v := uint64(metric.Value.(float64))
		return &v
	case string:
		v, err := strconv.ParseUint(metric.Value.(string), 10, 64)
		if err == nil {
			v2 := v
			return &v2
		}
		logger.Error().
			Str("metric", metricName).
			Interface("value", metric).
			Err(err).
			Msg("parsing")
	default:
		logger.Error().
			Str("metric", metricName).
			Interface("value", metric).
			Str("type", fmt.Sprintf("%T", t)).
			Msg("invalid value type for metric type")
	}
	return nil
}

func parseFloat(metricName string, metric tags.JSONMetric) *float64 {
	switch t := metric.Value.(type) {
	case float64:
		v := metric.Value.(float64)
		return &v
	case string:
		v, err := strconv.ParseFloat(metric.Value.(string), 64)
		if err == nil {
			v2 := v
			return &v2
		}
		logger.Error().
			Str("metric", metricName).
			Interface("value", metric).
			Err(err).
			Msg("parsing")
	default:
		logger.Error().
			Str("metric", metricName).
			Interface("value", metric).
			Str("type", fmt.Sprintf("%T", t)).
			Msg("invalid value type for metric type")
	}
	return nil
}

type histSample struct {
	bucket bool
	count  int64
	value  float64
}

func parseHistogram(metricName string, metric tags.JSONMetric) *[]histSample {
	switch t := metric.Value.(type) {
	case []interface{}:
		ret := make([]histSample, 0, len(metric.Value.([]interface{})))
		for idx, v := range metric.Value.([]interface{}) {
			switch t2 := v.(type) {
			case float64:
				ret = append(ret, histSample{bucket: false, value: v.(float64)})
			case string:
				sv := v.(string)
				switch {
				case strings.Contains(sv, "H["):
				// nothing, let the code below parse it
				case numFloatRx.MatchString(sv):
					v2, err := strconv.ParseFloat(sv, 64)
					if err != nil {
						logger.Error().
							Str("metric", metricName).
							Interface("value", v).
							Int("position", idx).
							Err(err).
							Msg("parsing histogram sample")
						continue
					}
					ret = append(ret, histSample{bucket: false, value: v2})
					continue
				default:
					// try to parse a serialized histogram
					raw := []byte(`"` + sv + `"`)
					var h circonusllhist.Histogram
					if err := json.Unmarshal(raw, &h); err != nil {
						logger.Error().
							Str("metric", metricName).
							Interface("value", v).
							Int("position", idx).
							Err(err).
							Msg("parsing serialized histogram")
						continue
					}
					sv = strings.Join(h.DecStrings(), " ")
				}

				//
				// it's an encoded histogram sample H[value]=count
				//
				matches := histogramRx.FindAllStringSubmatch(sv, -1)
				for _, match := range matches {
					bucket := ""
					count := ""
					for idx, val := range match {
						switch histogramRxNames[idx] {
						case "bucket":
							bucket = val
						case "count":
							count = val
						}
					}
					if bucket == "" || count == "" {
						logger.Error().
							Str("metric", metricName).
							Str("sample", sv).
							Int("position", idx).
							Msg("invalid encoded histogram sample")
						continue
					}
					b, err := strconv.ParseFloat(bucket, 64)
					if err != nil {
						logger.Error().
							Str("metric", metricName).
							Str("sample", sv).
							Int("position", idx).
							Err(err).
							Msg("encoded histogram sample, value parse")
						continue
					}
					c, err := strconv.ParseInt(count, 10, 64)
					if err != nil {
						logger.Error().
							Str("metric", metricName).
							Str("sample", sv).
							Int("position", idx).
							Err(err).
							Msg("encoded histogram sample, count parse")
						continue
					}
					ret = append(ret, histSample{bucket: true, value: b, count: c})
				}
			default:
				logger.Error().
					Str("metric", metricName).
					Interface("value", v).
					Int("position", idx).
					Str("type", fmt.Sprintf("%T", t2)).
					Msg("invalid value type for histogram sample")
			}
		}
		if len(ret) == 0 {
			return nil
		}
		return &ret
	default:
		logger.Error().
			Str("metric", metricName).
			Interface("value", metric).
			Str("type", fmt.Sprintf("%T", t)).
			Msg("invalid value type for histogram")
	}
	return nil
}
