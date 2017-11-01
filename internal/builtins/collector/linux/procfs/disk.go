// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package procfs

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	cgm "github.com/circonus-labs/circonus-gometrics"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// Disk metrics from the Linux ProcFS
type Disk struct {
	pfscommon
	include *regexp.Regexp
	exclude *regexp.Regexp
}

// diskOptions defines what elements can be overriden in a config file
type diskOptions struct {
	ID                   string   `json:"id" toml:"id" yaml:"id"`
	File                 string   `json:"proc_file" toml:"proc_file" yaml:"proc_file"`
	IncludeRegex         string   `json:"inlcude_regex" toml:"inlcude_regex" yaml:"inlcude_regex"`
	ExcludeRegex         string   `json:"exclude_regex" toml:"exclude_regex" yaml:"exclude_regex"`
	MetricsEnabled       []string `json:"metrics_enabled" toml:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsDisabled      []string `json:"metrics_disabled" toml:"metrics_disabled" yaml:"metrics_disabled"`
	MetricsDefaultStatus string   `json:"metrics_default_status" toml:"metrics_default_status" toml:"metrics_default_status"`
	RunTTL               string   `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`
}

type mdstat struct {
	id              string
	readsCompleted  uint64
	readsMerged     uint64
	sectorsRead     uint64
	readms          uint64
	writesCompleted uint64
	writesMerged    uint64
	sectorsWritten  uint64
	writems         uint64
	currIO          uint64
	ioms            uint64
	iomsWeighted    uint64
}

// NewDiskCollector creates new procfs cpu collector
func NewDiskCollector(cfgBaseName string) (collector.Collector, error) {
	c := Disk{}
	c.id = "disk"
	c.file = "/proc/diskstats"
	c.logger = log.With().Str("pkg", "procfs.disk").Logger()
	c.metricStatus = map[string]bool{}
	c.metricDefaultActive = true

	c.include = defaultIncludeRegex
	c.exclude = defaultExcludeRegex

	if cfgBaseName == "" {
		if _, err := os.Stat(c.file); os.IsNotExist(err) {
			return nil, errors.Wrap(err, "procfs.disk")
		}
		return &c, nil
	}

	var cfg diskOptions
	err := config.LoadConfigFile(cfgBaseName, &cfg)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, errors.Wrap(err, "procfs.disk config")
	}

	c.logger.Debug().Interface("config", cfg).Msg("loaded config")

	if cfg.File != "" {
		c.file = cfg.File
	}

	if cfg.IncludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, cfg.IncludeRegex))
		if err != nil {
			return nil, errors.Wrap(err, "procfs.disk compiling include regex")
		}
		c.include = rx
	}

	if cfg.ExcludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, cfg.ExcludeRegex))
		if err != nil {
			return nil, errors.Wrap(err, "procfs.disk compiling exclude regex")
		}
		c.exclude = rx
	}

	if cfg.ID != "" {
		c.id = cfg.ID
	}

	if len(cfg.MetricsEnabled) > 0 {
		for _, name := range cfg.MetricsEnabled {
			c.metricStatus[name] = true
		}
	}
	if len(cfg.MetricsDisabled) > 0 {
		for _, name := range cfg.MetricsDisabled {
			c.metricStatus[name] = false
		}
	}

	if cfg.MetricsDefaultStatus != "" {
		if ok, _ := regexp.MatchString(`^(enabled|disabled)$`, strings.ToLower(cfg.MetricsDefaultStatus)); ok {
			c.metricDefaultActive = strings.ToLower(cfg.MetricsDefaultStatus) == metricStatusEnabled
		} else {
			return nil, errors.Errorf("procfs.disk invalid metric default status (%s)", cfg.MetricsDefaultStatus)
		}
	}

	if cfg.RunTTL != "" {
		dur, err := time.ParseDuration(cfg.RunTTL)
		if err != nil {
			return nil, errors.Wrap(err, "procfs.disk parsing run_ttl")
		}
		c.runTTL = dur
	}

	if _, err := os.Stat(c.file); os.IsNotExist(err) {
		return nil, errors.Wrap(err, "procfs.disk")
	}

	return &c, nil
}

// Collect metrics from the procfs resource
func (c *Disk) Collect() error {
	metrics := cgm.Metrics{}

	c.Lock()

	if c.runTTL > time.Duration(0) {
		if time.Since(c.lastEnd) < c.runTTL {
			c.logger.Warn().Msg(collector.ErrTTLNotExpired.Error())
			c.Unlock()
			return collector.ErrTTLNotExpired
		}
	}
	if c.running {
		c.logger.Warn().Msg(collector.ErrAlreadyRunning.Error())
		c.Unlock()
		return collector.ErrAlreadyRunning
	}

	c.running = true
	c.lastStart = time.Now()
	c.Unlock()

	// aggregate stats for raids, if applicable
	mdList, mdStats := c.readmdstat()

	f, err := os.Open(c.file)
	if err != nil {
		c.setStatus(metrics, err)
		return errors.Wrap(err, "procfs.disk")
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	metricType := "L" // uint64

	for scanner.Scan() {

		line := scanner.Text()
		fields := strings.Fields(line)

		//  1 major                ignore
		//  2 minor                ignore
		//  3 device_name          apply include/exclude filters
		//  4 reads_completed      rd_completed
		//  5 reads_merged         rd_merged
		//  6 sectors_read         rd_sectors
		//  7 read_ms              rd_ms
		//  8 writes_completed     wr_completed
		//  9 writes_merged        wr_merged
		// 10 sectors_written      wr_sectors
		// 11 write_ms             wr_ms
		// 12 io_running           io_in_progress
		// 13 io_running_ms        io_ms
		// 14 weighted_io_ms       io_ms_weighted
		if len(fields) != 14 {
			continue
		}

		devName := fields[2]
		if c.exclude.MatchString(devName) || !c.include.MatchString(devName) {
			continue
		}

		pfx := c.id + metricNameSeparator + devName

		readsCompleted, err := strconv.ParseUint(fields[3], 10, 64)
		if err != nil {
			c.setStatus(metrics, err)
			return errors.Wrap(err, "procfs.disk parsing field reads completed")
		}
		c.addMetric(&metrics, pfx, "rd_completed", metricType, readsCompleted)

		readsMerged, err := strconv.ParseUint(fields[4], 10, 64)
		if err != nil {
			c.setStatus(metrics, err)
			return errors.Wrap(err, "procfs.disk parsing field reads merged")
		}
		c.addMetric(&metrics, pfx, "rd_merged", metricType, readsMerged)

		sectorsRead, err := strconv.ParseUint(fields[5], 10, 64)
		if err != nil {
			c.setStatus(metrics, err)
			return errors.Wrap(err, "procfs.disk parsing field sectors read")
		}
		c.addMetric(&metrics, pfx, "rd_sectors", metricType, sectorsRead)

		readms, err := strconv.ParseUint(fields[6], 10, 64)
		if err != nil {
			c.setStatus(metrics, err)
			return errors.Wrap(err, "procfs.disk parsing field read ms")
		}
		c.addMetric(&metrics, pfx, "rd_ms", metricType, readms)

		writesCompleted, err := strconv.ParseUint(fields[7], 10, 64)
		if err != nil {
			c.setStatus(metrics, err)
			return errors.Wrap(err, "procfs.disk parsing field writes completed")
		}
		c.addMetric(&metrics, pfx, "wr_completed", metricType, writesCompleted)

		writesMerged, err := strconv.ParseUint(fields[8], 10, 64)
		if err != nil {
			c.setStatus(metrics, err)
			return errors.Wrap(err, "procfs.disk parsing field writes merged")
		}
		c.addMetric(&metrics, pfx, "wr_merged", metricType, writesMerged)

		sectorsWritten, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			c.setStatus(metrics, err)
			return errors.Wrap(err, "procfs.disk parsing field sectors written")
		}
		c.addMetric(&metrics, pfx, "wr_sectors", metricType, sectorsWritten)

		writems, err := strconv.ParseUint(fields[10], 10, 64)
		if err != nil {
			c.setStatus(metrics, err)
			return errors.Wrap(err, "procfs.disk parsing field write ms")
		}
		c.addMetric(&metrics, pfx, "wr_ms", metricType, writems)

		currIO, err := strconv.ParseUint(fields[11], 10, 64)
		if err != nil {
			c.setStatus(metrics, err)
			return errors.Wrap(err, "procfs.disk parsing field IO ops in progress")
		}
		c.addMetric(&metrics, pfx, "io_in_progress", metricType, currIO)

		ioms, err := strconv.ParseUint(fields[12], 10, 64)
		if err != nil {
			c.setStatus(metrics, err)
			return errors.Wrap(err, "procfs.disk parsing field IO ms")
		}
		c.addMetric(&metrics, pfx, "io_ms", metricType, ioms)

		iomsWeighted, err := strconv.ParseUint(fields[13], 10, 64)
		if err != nil {
			c.setStatus(metrics, err)
			return errors.Wrap(err, "procfs.disk parsing field weighted IO ms")
		}
		c.addMetric(&metrics, pfx, "io_ms_weighted", metricType, iomsWeighted)

		if mdID, ok := mdList[devName]; ok {
			if md, ok := mdStats[mdID]; ok {
				md.readsCompleted += readsCompleted
				md.readsMerged += readsMerged
				md.sectorsRead += sectorsRead
				md.readms += readms

				md.writesCompleted += writesCompleted
				md.writesMerged += writesMerged
				md.sectorsWritten += sectorsWritten
				md.writems += writems

				md.currIO += currIO
				md.ioms += ioms
				md.iomsWeighted += iomsWeighted
			}
		}
	}

	if err := scanner.Err(); err != nil {
		c.setStatus(cgm.Metrics{}, err)
		return errors.Wrapf(err, "procfs.disk parsing %s", f.Name())
	}

	for _, md := range mdStats {
		pfx := c.id + metricNameSeparator + md.id
		c.addMetric(&metrics, pfx, "rd_completed", metricType, md.readsCompleted)
		c.addMetric(&metrics, pfx, "rd_merged", metricType, md.readsMerged)
		c.addMetric(&metrics, pfx, "rd_sectors", metricType, md.sectorsRead)
		c.addMetric(&metrics, pfx, "rd_ms", metricType, md.readms)
		c.addMetric(&metrics, pfx, "wr_completed", metricType, md.writesCompleted)
		c.addMetric(&metrics, pfx, "wr_merged", metricType, md.writesMerged)
		c.addMetric(&metrics, pfx, "wr_sectors", metricType, md.sectorsWritten)
		c.addMetric(&metrics, pfx, "wr_ms", metricType, md.writems)
		c.addMetric(&metrics, pfx, "io_in_progress", metricType, md.currIO)
		c.addMetric(&metrics, pfx, "io_ms", metricType, md.ioms)
		c.addMetric(&metrics, pfx, "io_ms_weighted", metricType, md.iomsWeighted)
	}

	c.setStatus(metrics, nil)
	return nil
}

func (c *Disk) readmdstat() (map[string]string, map[string]*mdstat) {
	mdstatPath := strings.Replace(c.file, "diskstats", "mdstat", -1)
	mdList := make(map[string]string)
	mdStats := make(map[string]*mdstat)
	f, err := os.Open(mdstatPath)
	if err != nil {
		return mdList, mdStats
	}
	defer f.Close()
	return mdList, mdStats
}
