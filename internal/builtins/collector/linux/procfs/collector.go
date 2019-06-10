// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +\build linux

package procfs

import (
	"io/ioutil"
	"regexp"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// common defines ProcFS metrics common elements
type common struct {
	id              string         // OPT id of the collector (used as metric name prefix)
	pkgID           string         // package prefix used for logging and errors
	procFSPath      string         // OPT procfs mount point path
	file            string         // the file in procfs
	lastEnd         time.Time      // last collection end time
	lastError       string         // last collection error
	lastMetrics     cgm.Metrics    // last metrics collected
	lastRunDuration time.Duration  // last collection duration
	lastStart       time.Time      // last collection start time
	logger          zerolog.Logger // collector logging instance
	metricNameChar  string         // OPT character(s) used as replacement for metricNameRegex
	metricNameRegex *regexp.Regexp // OPT regex for cleaning names, may be overridden in config
	running         bool           // is collector currently running
	runTTL          time.Duration  // OPT ttl for collectors (default is for every request)
	baseTags        tags.Tags
	sync.Mutex
}

// Define stubs to satisfy the collector.Collector interface.
//
// The individual collector implementations must override Collect and Flush.
//
// ID and Inventory are generic and do not need to be overridden unless the
// collector implementation requires it.

// Collect returns collector metrics
func (c *common) Collect() error {
	c.Lock()
	defer c.Unlock()
	return collector.ErrNotImplemented
}

// Flush returns last metrics collected
func (c *common) Flush() cgm.Metrics {
	c.Lock()
	defer c.Unlock()
	if c.lastMetrics == nil {
		c.lastMetrics = cgm.Metrics{}
	}
	return c.lastMetrics
}

// ID returns the id of the instance
func (c *common) ID() string {
	c.Lock()
	defer c.Unlock()
	return c.id
}

// Inventory returns collector stats for /inventory endpoint
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

// Logger returns collector's instance of logger
func (c *common) Logger() zerolog.Logger {
	return c.logger
}

// cleanName is used to clean the metric name
func (c *common) cleanName(name string) string {
	// metric names are not dynamic for linux procfs - reintroduce cleaner if
	// procfs sources used return dirty dynamic names.
	//
	// return c.metricNameRegex.ReplaceAllString(name, c.metricNameChar)
	return name
}

// addMetric to internal buffer if metric is active
func (c *common) addMetric(metrics *cgm.Metrics, prefix string, mname, mtype string, mval interface{}, mtags tags.Tags) error {
	if metrics == nil {
		return errors.New("invalid metric submission")
	}

	if mname == "" {
		return errors.New("invalid metric, no name")
	}

	if mtype == "" {
		return errors.New("invalid metric, no type")
	}

	// cleanup the raw metric name, if needed
	mname = c.cleanName(mname)
	metricName := mname
	if prefix != "" {
		metricName = prefix + metricNameSeparator + mname
	}

	var tagList tags.Tags
	tagList = append(tagList, c.baseTags...)
	tagList = append(tagList, tags.Tags{
		tags.Tag{Category: "source", Value: release.NAME},
		tags.Tag{Category: "collector", Value: c.id},
	}...)
	tagList = append(tagList, mtags...)

	metricName = tags.MetricNameWithStreamTags(metricName, tagList)
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

func (c *common) readFile(file string) ([]byte, error) {
	if file == "" {
		return nil, errors.New("invalid file (empty)")
	}
	return ioutil.ReadFile(file)
}
