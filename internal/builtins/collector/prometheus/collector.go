// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package prometheus

import (
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/rs/zerolog"
)

// Flush returns last metrics collected.
func (c *Prom) Flush() cgm.Metrics {
	c.Lock()
	defer c.Unlock()
	if c.lastMetrics == nil {
		c.lastMetrics = cgm.Metrics{}
	}
	return c.lastMetrics
}

// ID returns the id of the instance.
func (c *Prom) ID() string {
	return "promfetch"
}

// Inventory returns collector stats for /inventory endpoint.
func (c *Prom) Inventory() collector.InventoryStats {
	c.Lock()
	defer c.Unlock()
	return collector.InventoryStats{
		ID:              "promfetch",
		LastRunStart:    c.lastStart.Format(time.RFC3339Nano),
		LastRunEnd:      c.lastEnd.Format(time.RFC3339Nano),
		LastRunDuration: c.lastRunDuration.String(),
		LastError:       c.lastError,
	}
}

// Logger returns collector's instance of logger.
func (c *Prom) Logger() zerolog.Logger {
	return c.logger
}

// cleanName is used to clean the metric name.
func (c *Prom) cleanName(name string) string {
	return c.metricNameRegex.ReplaceAllString(name, "")
}

// addMetric to internal buffer if metric is active.
func (c *Prom) addMetric(metrics *cgm.Metrics, prefix string, mname string, mtags tags.Tags, mtype string, mval interface{}) error {
	if metrics == nil {
		return errInvalidMetric
	}

	if mname == "" {
		return errInvalidMetricNoName
	}

	if mtype == "" {
		return errInvalidMetricNoType
	}

	// cleanup the raw metric name, if needed
	metricName := c.cleanName(mname)
	if prefix != "" {
		metricName = prefix + config.MetricNameSeparator + metricName
	}

	var tagList tags.Tags
	tagList = append(tagList, tags.Tags{
		tags.Tag{Category: "source", Value: release.NAME},
		tags.Tag{Category: "collector", Value: "promfetch"},
	}...)
	tagList = append(tagList, mtags...)

	// Add stream tags
	metricName = tags.MetricNameWithStreamTags(metricName, tagList)

	(*metrics)[metricName] = cgm.Metric{Type: mtype, Value: mval}
	return nil
}

// setStatus is used in Collect to set the collector status.
func (c *Prom) setStatus(metrics cgm.Metrics, err error) {
	c.Lock()
	if err == nil {
		c.lastError = ""
		c.lastMetrics = metrics
	} else {
		c.lastError = err.Error()
		// on error, ensure metrics are reset
		// do not keep returning a stale set of metrics
		c.lastMetrics = cgm.Metrics{}
	}
	c.lastEnd = time.Now()
	if !c.lastStart.IsZero() {
		c.lastRunDuration = time.Since(c.lastStart)
	}
	c.running = false
	c.Unlock()
}
