// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
	"context"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/rs/zerolog"
)

// gencommon defines psutils metrics common elements.
type gencommon struct {
	id              string         // OPT id of the collector (used as metric name prefix)
	pkgID           string         // package prefix used for logging and errors
	lastError       string         // last collection error
	baseTags        tags.Tags      // base tags
	lastMetrics     cgm.Metrics    // last metrics collected
	lastEnd         time.Time      // last collection end time
	lastStart       time.Time      // last collection start time
	logger          zerolog.Logger // collector logging instance
	lastRunDuration time.Duration  // last collection duration
	runTTL          time.Duration  // OPT ttl for collectors (default is for every request)
	running         bool           // is collector currently running
	sync.Mutex
}

// Collect returns collector metrics.
func (c *gencommon) Collect(_ context.Context) error {
	c.Lock()
	defer c.Unlock()
	return collector.ErrNotImplemented
}

// Flush returns last metrics collected.
func (c *gencommon) Flush() cgm.Metrics {
	c.Lock()
	defer c.Unlock()
	if c.lastMetrics == nil {
		c.lastMetrics = cgm.Metrics{}
	}
	return c.lastMetrics
}

// ID returns the id of the instance.
func (c *gencommon) ID() string {
	c.Lock()
	defer c.Unlock()
	if c.id == "" {
		return c.pkgID
	}
	return c.id
}

// TTL return run TTL if set.
func (c *gencommon) TTL() string {
	c.Lock()
	defer c.Unlock()
	if c.runTTL == time.Duration(0) {
		return ""
	}
	return c.runTTL.String()
}

// Inventory returns collector stats for /inventory endpoint.
func (c *gencommon) Inventory() collector.InventoryStats {
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
func (c *gencommon) Logger() zerolog.Logger {
	return c.logger
}

// addMetric to internal buffer if metric is active.
func (c *gencommon) addMetric(metrics *cgm.Metrics, mname, mtype string, mval interface{}, mtags tags.Tags) error {
	if metrics == nil {
		return errInvalidMetric
	}

	if mname == "" {
		return errInvalidMetricNoName
	}

	if mtype == "" {
		return errInvalidMetricNoType
	}

	var tagList tags.Tags
	tagList = append(tagList, c.baseTags...)
	tagList = append(tagList, tags.Tags{
		tags.Tag{Category: "source", Value: release.NAME},
		tags.Tag{Category: "collector", Value: c.id},
	}...)
	tagList = append(tagList, mtags...)

	metricName := tags.MetricNameWithStreamTags(mname, tagList)

	(*metrics)[metricName] = cgm.Metric{Type: mtype, Value: mval}

	return nil
}

// setStatus is used in Collect to set the collector status.
func (c *gencommon) setStatus(metrics cgm.Metrics, err error) {
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
