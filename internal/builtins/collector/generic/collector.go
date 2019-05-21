// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package generic

import (
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// gencommon defines psutils metrics common elements
type gencommon struct {
	id              string         // OPT id of the collector (used as metric name prefix)
	pkgID           string         // package prefix used for logging and errors
	lastEnd         time.Time      // last collection end time
	lastError       string         // last collection error
	lastMetrics     cgm.Metrics    // last metrics collected
	lastRunDuration time.Duration  // last collection duration
	lastStart       time.Time      // last collection start time
	logger          zerolog.Logger // collector logging instance
	running         bool           // is collector currently running
	runTTL          time.Duration  // OPT ttl for collectors (default is for every request)
	baseTags        tags.Tags
	sync.Mutex
}

// Collect returns collector metrics
func (c *gencommon) Collect() error {
	c.Lock()
	defer c.Unlock()
	return collector.ErrNotImplemented
}

// Flush returns last metrics collected
func (c *gencommon) Flush() cgm.Metrics {
	c.Lock()
	defer c.Unlock()
	if c.lastMetrics == nil {
		c.lastMetrics = cgm.Metrics{}
	}
	return c.lastMetrics
}

// ID returns the id of the instance
func (c *gencommon) ID() string {
	c.Lock()
	defer c.Unlock()
	if c.id == "" {
		return c.pkgID
	}
	return c.id
}

// TTL return run TTL if set
func (c *gencommon) TTL() string {
	c.Lock()
	defer c.Unlock()
	if c.runTTL == time.Duration(0) {
		return ""
	}
	return c.runTTL.String()
}

// Inventory returns collector stats for /inventory endpoint
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

// Logger returns collector's instance of logger
func (c *gencommon) Logger() zerolog.Logger {
	return c.logger
}

// addMetric to internal buffer if metric is active
func (c *gencommon) addMetric(metrics *cgm.Metrics, mname, mtype string, mval interface{}, mtags tags.Tags) error {
	if metrics == nil {
		return errors.New("invalid metric submission")
	}

	if mname == "" {
		return errors.New("invalid metric, no name")
	}

	if mtype == "" {
		return errors.New("invalid metric, no type")
	}

	var tagList tags.Tags
	tagList = append(tagList, c.baseTags...)
	tagList = append(tagList, mtags...)

	metricName := tags.MetricNameWithStreamTags(mname, tagList)

	(*metrics)[metricName] = cgm.Metric{Type: mtype, Value: mval}

	return nil
}

// setStatus is used in Collect to set the collector status
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
