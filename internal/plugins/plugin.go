// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package plugins

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
)

// drain returns and resets plugin's current metrics
func (p *plugin) drain() *cgm.Metrics {
	p.Lock()
	defer p.Unlock()

	var metrics *cgm.Metrics
	if p.metrics == nil {
		if p.prevMetrics == nil {
			metrics = &cgm.Metrics{}
		} else {
			metrics = p.prevMetrics
		}
	} else {
		metrics = p.metrics
		p.metrics = nil
		p.prevMetrics = metrics
	}

	return metrics
}

// baseTagList returns the base tags for the plugin
func (p *plugin) baseTagList() []string {
	tagList := []string{
		"source:" + release.NAME,
		"collector:" + p.id,
	}
	if p.instanceID != "" {
		tagList = append(tagList, "instance:"+p.instanceID)
	}
	tagList = append(tagList, p.baseTags...)

	return tagList
}

// parsePluginOutput handles json and tab delimited output from plugins.
func (p *plugin) parsePluginOutput(output []string) error {
	p.Lock()
	defer p.Unlock()

	if len(output) == 0 {
		p.metrics = &cgm.Metrics{}
		return errors.Errorf("zero lines of output")
	}

	parseStart := time.Now()
	metrics := cgm.Metrics{}
	numDuplicates := 0

	// if first char of first line is '{' then assume output is json
	if output[0][:1] == "{" {
		var jm tags.JSONMetrics
		err := json.Unmarshal([]byte(strings.Join(output, "\n")), &jm)
		if err != nil {
			p.logger.Error().
				Err(err).
				Str("output", strings.Join(output, "\n")).
				Msg("parsing json")
			p.metrics = &cgm.Metrics{}
			return errors.Wrap(err, "parsing json")
		}
		for mn, md := range jm {
			// add stream tags to metric name
			tagList := p.baseTagList()
			tagList = append(tagList, md.Tags...)
			metrics[tags.MetricNameWithStreamTags(mn, tags.FromList(tagList))] = cgm.Metric{Type: md.Type, Value: md.Value}
		}
		p.metrics = &metrics
		return nil
	}

	// otherwise, assume it is delimited fields:
	//  fieldDelimiter is current TAB
	//  metric_name<TAB>metric_type[<TAB>metric_value<TAB>tags]
	//  foo\ti\t10  - int32 foo w/value 10
	//  bar\tL      - uint64 bar w/o value (null, metric is present but has no value)
	// note: tags is a comma separated list of key:value pairs (e.g. foo:bar,cat:dog)
	metricTypes := regexp.MustCompile("^[iIlLnOs]$")
	for _, line := range output {
		delimCount := strings.Count(line, fieldDelimiter)
		if delimCount == 0 {
			p.logger.Error().
				Str("line", line).
				Msg("invalid format, zero field delimiters found")
			continue
		}

		fields := strings.Split(line, fieldDelimiter)
		if len(fields) <= 1 || len(fields) > 4 {
			p.logger.Error().
				Str("line", line).
				Int("fields", len(fields)).
				Int("delimiters", delimCount).
				Msg("invalid number of fields - expect 2, 3, or 4")
			continue
		}

		metricName := strings.Replace(fields[0], " ", metricDelimiter, -1)
		metricType := fields[1]

		if _, ok := metrics[metricName]; ok {
			p.logger.Warn().Str("name", metricName).Msg("duplicate name, skipping")
			numDuplicates++
			continue
		}

		if !metricTypes.MatchString(metricType) {
			p.logger.Error().
				Str("line", line).
				Str("type", metricType).
				Msg("invalid metric type")
			continue
		}

		// only received a name and type (intentionally null value)
		if len(fields) == 2 {
			metrics[tags.MetricNameWithStreamTags(metricName, tags.FromList(p.baseTags))] = cgm.Metric{
				Type:  metricType,
				Value: nullMetricValue,
			}
			continue
		}

		metricValue := fields[2]

		// add stream tags to metric name
		metricTags := []string{}
		if len(fields) == 4 {
			metricTags = strings.Split(fields[3], tags.Separator)
		}
		tagList := p.baseTagList()
		tagList = append(tagList, metricTags...)
		metricName = tags.MetricNameWithStreamTags(metricName, tags.FromList(tagList))

		// intentionally null value, explicit syntax
		if strings.ToLower(metricValue) == nullMetricValue {
			metrics[metricName] = cgm.Metric{
				Type:  metricType,
				Value: nullMetricValue,
			}
			continue
		}

		metric := cgm.Metric{}

		switch metricType {
		case "i": // signed 32bit
			metric.Type = metricType
			i, err := strconv.ParseInt(metricValue, 10, 32)
			if err != nil {
				p.logger.Error().
					Err(err).
					Str("line", line).
					Msg("unable to parse int32")
				continue
			}
			metric.Value = int32(i)
		case "I": // unsigned 32bit
			metric.Type = metricType
			u, err := strconv.ParseUint(metricValue, 10, 32)
			if err != nil {
				p.logger.Error().
					Err(err).
					Str("line", line).
					Msg("unable to parse uint32")
				continue
			}
			metric.Value = uint32(u)
		case "l": // signed 64bit
			metric.Type = metricType
			i, err := strconv.ParseInt(metricValue, 10, 64)
			if err != nil {
				p.logger.Error().
					Err(err).
					Str("line", line).
					Msg("unable to parse int64")
				continue
			}
			metric.Value = i
		case "L": // unsigned 64bit
			metric.Type = metricType
			u, err := strconv.ParseUint(metricValue, 10, 64)
			if err != nil {
				p.logger.Error().
					Err(err).
					Str("line", line).
					Msg("unable to parse uint64")
				continue
			}
			metric.Value = u
		case "n": // double
			metric.Type = metricType
			f, err := strconv.ParseFloat(metricValue, 64)
			if err != nil {
				p.logger.Error().
					Err(err).
					Str("line", line).
					Msg("unable to parse double/float")
				continue
			}
			metric.Value = f
		case "s": // string
			metric.Type = metricType
			metric.Value = metricValue
		case "O": // have Circonus automatically detect
			metric.Type = metricType
			metric.Value = metricValue
		default:
			p.logger.Error().
				Str("line", line).
				Str("type", metricType).
				Msg("unknown metric type")
			continue
		}

		metrics[metricName] = metric
	}

	p.logger.Debug().
		Str("duration", time.Since(parseStart).String()).
		Int("lines", len(output)).
		Int("metrics", len(metrics)).
		Int("duplicates", numDuplicates).
		Int("errors", len(output)-(len(metrics)+numDuplicates)).
		Msg("processed plugin output")

	p.metrics = &metrics

	return nil
}

// exec runs a specific plugin and saves plugin output
func (p *plugin) exec() error {
	// NOTE: !! IMPORTANT !!
	//       locks are handled manually so that long running plugins
	//       do not block access to plugin meta data and metrics
	p.Lock()

	p.currStart = time.Now()
	plog := p.logger

	//plog.Debug().Msg("running")

	if p.runTTL > time.Duration(0) {
		if time.Since(p.lastEnd) < p.runTTL {
			msg := "TTL not expired"
			plog.Debug().Msg(msg)
			p.Unlock()
			return errors.New(msg)
		}
	}

	if p.running {
		msg := "already running"
		plog.Debug().Msg(msg)
		p.Unlock()
		return errors.New(msg)
	}

	plog.Debug().Msg("start")
	p.running = true
	// TBD: timeouts, create a new deadline context
	//      Problem is some plugins do not exit intentionally - long running.
	//      There is no way [currently] to know whether a plugin is
	//      intentionally "long running".
	p.cmd = exec.CommandContext(p.ctx, p.command)
	p.cmd.Dir = p.runDir
	if p.instanceArgs != nil {
		p.cmd.Args = append(p.cmd.Args, p.instanceArgs...)
	}

	var errOut bytes.Buffer
	p.cmd.Stderr = &errOut

	p.Unlock()

	resetStatus := func(err error) {
		p.Lock()
		p.lastStart = p.currStart
		p.lastEnd = time.Now()
		p.lastRunDuration = time.Since(p.lastStart)
		p.lastError = err
		p.running = false
		p.Unlock()
	}

	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		msg := "stdout pipe"
		plog.Error().
			Err(err).
			Msg(msg)
		resetStatus(err)
		return errors.Wrap(err, msg)
	}

	lines := []string{}
	scanner := bufio.NewScanner(stdout)

	if err := p.cmd.Start(); err != nil {
		msg := "cmd start"
		plog.Error().
			Err(err).
			Str("cmd", p.command).
			Msg(msg)
		resetStatus(err)
		return errors.Wrap(err, msg)
	}

	for scanner.Scan() {
		line := scanner.Text()

		// blank line, long running plugin signal to parse
		// what has already been received.
		if line == "" {
			p.parsePluginOutput(lines)
			lines = []string{}
			continue
		}

		// add line to buffer for processing
		lines = append(lines, line)
	}

	var runErr error

	if err := scanner.Err(); err != nil {
		plog.Error().
			Err(err).
			Msg("reading stdio")

		runErr = errors.Wrap(err, "scanner, reading stdio")
	}

	// parse lines if there are any in the buffer
	// or, in case of long running plugin, any left in buffer on exit
	p.parsePluginOutput(lines)

	if err := p.cmd.Wait(); err != nil {
		var stderr string
		if errOut.Len() > 0 {
			stderr = strings.Replace(errOut.String(), "\n", "", -1)
		}
		if exiterr, ok := err.(*exec.ExitError); ok {
			errMsg := fmt.Sprintf("%s %s", stderr, exiterr.Stderr)
			plog.Error().
				Str("stderr", errMsg).
				Str("status", exiterr.String()).
				Str("cmd", p.command).
				Msg("exited non-zero")
			if runErr != nil {
				runErr = errors.Wrapf(exiterr, "cmd err (%s) and %s", errMsg, runErr)
			} else {
				runErr = errors.Wrapf(exiterr, "cmd err (%s)", errMsg)
			}
		} else {
			plog.Error().
				Err(err).
				Str("cmd", p.command).
				Str("stderr", stderr).
				Msg("exited non-zero (not exiterr)")
			if runErr != nil {
				runErr = errors.Wrapf(err, "cmd err (%s) and %s", stderr, runErr)
			} else {
				runErr = errors.Wrapf(err, "cmd err (%s)", stderr)
			}
		}
	}

	resetStatus(runErr)
	return runErr
}
