// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

//go:build windows
// +build windows

package nvidia

import (
	"context"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/rs/zerolog"
)

// Define stubs to satisfy the collector.Collector interface.
//
// The individual wmi collector implementations must override Collect and Flush.
//
// ID and Inventory are generic and do not need to be overridden unless the
// collector implementation requires it.

// Collect metrics.
func (c *common) Collect(_ context.Context) error {
	c.Lock()
	defer c.Unlock()
	return collector.ErrNotImplemented
}

// Flush returns last metrics collected.
func (c *common) Flush() cgm.Metrics {
	c.Lock()
	defer c.Unlock()
	metrics := c.metrics.FlushMetrics()
	if c.lastMetrics == nil {
		c.lastMetrics = *metrics
	}
	return *metrics
}

// ID returns id of collector.
func (c *common) ID() string {
	c.Lock()
	defer c.Unlock()
	return c.id
}

// Inventory returns collector stats for /inventory endpoint.
func (c *common) Inventory() collector.InventoryStats {
	c.Lock()
	defer c.Unlock()
	return collector.InventoryStats{
		ID:              c.id,
		LastRunStart:    c.lastStart.Format(time.RFC3339Nano),
		LastRunEnd:      c.lastEnd.Format(time.RFC3339Nano),
		LastRunDuration: c.lastRunDuration.String(),
		LastError:       c.lastError,
	}
}

// Logger returns collector's instance of logger.
func (c *common) Logger() zerolog.Logger {
	return c.logger
}

/*
// cleanName is used to clean the metric name
func (c *common) cleanName(name string) string {
	return c.metricNameRegex.ReplaceAllString(name, c.metricNameChar)
}

// addMetric to internal buffer if metric is active
func (c *common) addMetric(metrics *cgm.Metrics, pfx, mname, mtype string, mval interface{}, mtags cgm.Tags) error {
	if metrics == nil {
		return errors.New("invalid metric submission")
	}

	if mname == "" {
		return errors.New("invalid metric, no name")
	}

	if mtype == "" {
		return errors.New("invalid metric, no type")
	}

	var tagList cgm.Tags
	tagList = append(tagList, cgm.Tags{
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "collector", Value: c.id},
	}...)
	tagList = append(tagList, c.baseTags...)
	tagList = append(tagList, mtags...)

	if pfx != "" {
		mname = pfx + defaults.MetricNameSeparator + mname
	}

	metricName := tags.MetricNameWithStreamTags(c.cleanName(mname), tagList)
	(*metrics)[metricName] = cgm.Metric{Type: mtype, Value: mval}

	return nil
}

// setStatus is used in Collect to set the collector status
func (c *common) setStatus(metrics cgm.Metrics, err error) {
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
*/
