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

type dstats struct {
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

	f, err := os.Open(c.file)
	if err != nil {
		c.setStatus(metrics, err)
		return errors.Wrap(err, "procfs.disk")
	}
	defer f.Close()

	var stats map[string]*dstats
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {

		line := strings.TrimSpace(scanner.Text())
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

		ds, err := c.parse(fields)
		if err != nil {
			continue // parser logs error
		}
		stats[ds.id] = ds
	}

	if err := scanner.Err(); err != nil {
		c.setStatus(cgm.Metrics{}, err)
		return errors.Wrapf(err, "procfs.disk parsing %s", f.Name())
	}

	// get list of devices for each entry in mdstats (if it exists)
	mdList := c.parsemdstat()
	mdrx := regexp.MustCompile(`^md[0-9]+`)
	pfx := c.id + metricNameSeparator
	metricType := "L" // uint64
	for devID, devStats := range stats {
		if mdrx.MatchString(devID) { // is it an md device?
			if devList, ok := mdList[devID]; ok { // have device list for it?
				for _, dev := range devList {
					if ds, found := stats[dev]; found { // have stats for the device?
						//
						// original diskstats.sh only aggregates a subset of metrics
						//
						// unclear why _only_ these specific metrics, seems logical that
						// it would be an all or nothing scenario.
						//
						// seeking documentation supporting only the aggregation
						// of a subset of the metrics

						// devStats.readsCompleted += ds.readsCompleted
						// devStats.readsMerged += ds.readsMerged
						// devStats.sectorsRead += ds.sectorsRead
						devStats.readms += ds.readms

						// devStats.writesCompleted += ds.writesCompleted
						// devStats.writesMerged += ds.writesMerged
						// devStats.sectorsWritten += ds.sectorsWritten
						devStats.writems += ds.writems

						devStats.currIO += ds.currIO
						devStats.ioms += ds.ioms
						// devStats.iomsWeighted += ds.iomsWeighted
					}
				}
			}
		}

		// do the exclusions here, if the device is part of an md its
		// metrics need to be included in the aggregation
		if c.exclude.MatchString(devID) || !c.include.MatchString(devID) {
			c.logger.Debug().Str("device", devID).Msg("excluded device name, ignoring")
			continue
		}

		c.addMetric(&metrics, pfx+devID, "rd_completed", metricType, devStats.readsCompleted)
		c.addMetric(&metrics, pfx+devID, "rd_merged", metricType, devStats.readsMerged)
		c.addMetric(&metrics, pfx+devID, "rd_sectors", metricType, devStats.sectorsRead)
		c.addMetric(&metrics, pfx+devID, "rd_ms", metricType, devStats.readms)
		c.addMetric(&metrics, pfx+devID, "wr_completed", metricType, devStats.writesCompleted)
		c.addMetric(&metrics, pfx+devID, "wr_merged", metricType, devStats.writesMerged)
		c.addMetric(&metrics, pfx+devID, "wr_sectors", metricType, devStats.sectorsWritten)
		c.addMetric(&metrics, pfx+devID, "wr_ms", metricType, devStats.writems)
		c.addMetric(&metrics, pfx+devID, "io_in_progress", metricType, devStats.currIO)
		c.addMetric(&metrics, pfx+devID, "io_ms", metricType, devStats.ioms)
		c.addMetric(&metrics, pfx+devID, "io_ms_weighted", metricType, devStats.iomsWeighted)
	}

	c.setStatus(metrics, nil)
	return nil
}

func (c *Disk) parse(fields []string) (*dstats, error) {
	devName := fields[2]
	if devName == "" {
		c.logger.Debug().Msg("invalid device name (empty), ignoring")
		return nil, errors.New("invalid device name (empty)")
	}

	pe := errors.New("parsing field")
	d := dstats{
		id: devName,
	}

	if v, err := strconv.ParseUint(fields[3], 10, 64); err == nil {
		d.readsCompleted = v
	} else {
		c.logger.Warn().Err(err).Str("dev", devName).Msg("parsing field reads completed")
		return nil, pe
	}

	if v, err := strconv.ParseUint(fields[4], 10, 64); err == nil {
		d.readsMerged = v
	} else {
		c.logger.Warn().Err(err).Str("dev", devName).Msg("parsing field reads merged")
		return nil, pe
	}

	if v, err := strconv.ParseUint(fields[5], 10, 64); err == nil {
		d.sectorsRead = v
	} else {
		c.logger.Warn().Err(err).Str("dev", devName).Msg("parsing field sectors read")
		return nil, pe
	}

	if v, err := strconv.ParseUint(fields[6], 10, 64); err == nil {
		d.readms = v
	} else {
		c.logger.Warn().Err(err).Str("dev", devName).Msg("parsing field read ms")
		return nil, pe
	}

	if v, err := strconv.ParseUint(fields[7], 10, 64); err == nil {
		d.writesCompleted = v
	} else {
		c.logger.Warn().Err(err).Str("dev", devName).Msg("parsing field writes completed")
		return nil, pe
	}

	if v, err := strconv.ParseUint(fields[8], 10, 64); err == nil {
		d.writesMerged = v
	} else {
		c.logger.Warn().Err(err).Str("dev", devName).Msg("parsing field writes merged")
		return nil, pe
	}

	if v, err := strconv.ParseUint(fields[9], 10, 64); err == nil {
		d.sectorsWritten = v
	} else {
		c.logger.Warn().Err(err).Str("dev", devName).Msg("parsing field sectors written")
		return nil, pe
	}

	if v, err := strconv.ParseUint(fields[10], 10, 64); err == nil {
		d.writems = v
	} else {
		c.logger.Warn().Err(err).Str("dev", devName).Msg("parsing field write ms")
		return nil, pe
	}

	if v, err := strconv.ParseUint(fields[11], 10, 64); err == nil {
		d.currIO = v
	} else {
		c.logger.Warn().Err(err).Str("dev", devName).Msg("parsing field IO ops in progress")
		return nil, pe
	}

	if v, err := strconv.ParseUint(fields[12], 10, 64); err == nil {
		d.ioms = v
	} else {
		c.logger.Warn().Err(err).Str("dev", devName).Msg("parsing field IO ms")
		return nil, pe
	}

	if v, err := strconv.ParseUint(fields[13], 10, 64); err == nil {
		d.iomsWeighted = v
	} else {
		c.logger.Warn().Err(err).Str("dev", devName).Msg("parsing field weighted IO ms")
		return nil, pe
	}

	return &d, nil
}

func (c *Disk) parsemdstat() map[string][]string {
	mdstatPath := strings.Replace(c.file, "diskstats", "mdstat", -1)
	mdList := make(map[string][]string)
	f, err := os.Open(mdstatPath)
	if err != nil {
		c.logger.Debug().Err(err).Str("mdstats", mdstatPath).Msg("loading mdstats, ignoring")
		return mdList
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	mdrx := regexp.MustCompile(`^md[0-9]+`)
	devrx := regexp.MustCompile(`^([^\[]+)\[.+$`)
	for scanner.Scan() {

		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)

		if !mdrx.MatchString(fields[0]) {
			continue
		}
		mdID := fields[0]
		// md0 : active raid1 sdb1[1] sda1[0]
		// 1 md name
		// 2 colon
		// 3 status
		// 4 type
		// 5+ devices
		if len(fields) < 5 {
			continue
		}

		// extract list of devices in the md
		devList := []string{}
		for i := 4; i < len(fields); i++ {
			devID := devrx.ReplaceAllString(fields[i], `$1`)
			devList = append(devList, devID)
		}

		mdList[mdID] = devList
	}

	if err := scanner.Err(); err != nil {
		c.logger.Debug().Err(err).Str("mdstats", mdstatPath).Msg("loading mdstats, ignoring")
		return map[string][]string{}
	}
	return mdList
}
