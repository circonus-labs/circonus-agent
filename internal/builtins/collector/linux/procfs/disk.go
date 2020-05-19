// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build linux

package procfs

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// Disk metrics from the Linux ProcFS
type Disk struct {
	common
	include           *regexp.Regexp
	exclude           *regexp.Regexp
	sectorSizeDefault uint64
	sectorSizeCache   map[string]uint64
}

// diskOptions defines what elements can be overridden in a config file
type diskOptions struct {
	// common
	ID         string `json:"id" toml:"id" yaml:"id"`
	ProcFSPath string `json:"procfs_path" toml:"procfs_path" yaml:"procfs_path"`
	RunTTL     string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`

	// collector specific
	IncludeRegex      string `json:"include_regex" toml:"include_regex" yaml:"include_regex"`
	ExcludeRegex      string `json:"exclude_regex" toml:"exclude_regex" yaml:"exclude_regex"`
	DefaultSectorSize string `json:"default_sector_size" toml:"default_sector_size" yaml:"default_sector_size"`
}

type dstats struct {
	id                string
	readsCompleted    uint64
	readsMerged       uint64
	sectorsRead       uint64
	bytesRead         uint64
	readms            uint64
	writesCompleted   uint64
	writesMerged      uint64
	sectorsWritten    uint64
	bytesWritten      uint64
	writems           uint64
	currIO            uint64
	ioms              uint64
	iomsWeighted      uint64
	haveKernel418     bool
	discardsCompleted uint64
	discardsMerged    uint64
	sectorsDiscarded  uint64
	discardms         uint64
}

// NewDiskCollector creates new procfs disk collector
func NewDiskCollector(cfgBaseName, procFSPath string) (collector.Collector, error) {
	procFile := "diskstats"

	c := Disk{
		common: newCommon(NameDisk, procFSPath, procFile, tags.FromList(tags.GetBaseTags())),
	}

	c.sectorSizeCache = make(map[string]uint64)
	c.include = defaultIncludeRegex
	c.exclude = defaultExcludeRegex
	c.sectorSizeDefault = 512

	if cfgBaseName == "" {
		if _, err := os.Stat(c.file); os.IsNotExist(err) {
			return nil, errors.Wrap(err, c.pkgID)
		}
		return &c, nil
	}

	var opts diskOptions
	err := config.LoadConfigFile(cfgBaseName, &opts)
	if err != nil {
		if !strings.Contains(err.Error(), "no config found matching") {
			c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
			return nil, errors.Wrapf(err, "%s config", c.pkgID)
		}
	} else {
		c.logger.Debug().Str("base", cfgBaseName).Interface("config", opts).Msg("loaded config")
	}

	if opts.IncludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, opts.IncludeRegex))
		if err != nil {
			return nil, errors.Wrapf(err, "%s compiling include regex", c.pkgID)
		}
		c.include = rx
	}

	if opts.ExcludeRegex != "" {
		rx, err := regexp.Compile(fmt.Sprintf(regexPat, opts.ExcludeRegex))
		if err != nil {
			return nil, errors.Wrapf(err, "%s compiling exclude regex", c.pkgID)
		}
		c.exclude = rx
	}

	if opts.DefaultSectorSize != "" {
		v, err := strconv.ParseUint(opts.DefaultSectorSize, 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "%s parsing default sector size", c.pkgID)
		}
		c.sectorSizeDefault = v
	}

	if opts.ID != "" {
		c.id = opts.ID
	}

	if opts.ProcFSPath != "" {
		c.procFSPath = opts.ProcFSPath
		c.file = filepath.Join(c.procFSPath, procFile)
	}

	if opts.RunTTL != "" {
		dur, err := time.ParseDuration(opts.RunTTL)
		if err != nil {
			return nil, errors.Wrapf(err, "%s parsing run_ttl", c.pkgID)
		}
		c.runTTL = dur
	}

	if _, err := os.Stat(c.file); os.IsNotExist(err) {
		return nil, errors.Wrap(err, c.pkgID)
	}

	return &c, nil
}

// Collect metrics from the procfs resource
func (c *Disk) Collect(ctx context.Context) error {
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

	stats := make(map[string]*dstats)

	lines, err := c.readFile(c.file)
	if err != nil {
		c.setStatus(metrics, err)
		return errors.Wrap(err, c.pkgID)
	}
	for _, line := range lines {
		fields := strings.Fields(line)

		//  0 major                ignore
		//  1 minor                ignore
		//  2 device_name          apply include/exclude filters
		// https://github.com/torvalds/linux/blob/master/Documentation/admin-guide/iostats.rst
		// NOTE: in the doc, field 1 is 4 here, reads completed
		//  3 reads_completed      rd_completed
		//  4 reads_merged         rd_merged
		//  5 sectors_read         rd_sectors
		//  6 read_ms              rd_ms
		//  7 writes_completed     wr_completed
		//  8 writes_merged        wr_merged
		//  9 sectors_written      wr_sectors
		// 10 write_ms             wr_ms
		// 11 io_running           io_in_progress
		// 12 io_running_ms        io_ms
		// 13 weighted_io_ms       io_ms_weighted
		// Kernel 4.18+ adds:
		// 14 - discards completed successfully
		// 15 - discards merged
		// 16 - sectors discarded
		// 17 - time spent discarding (milliseconds)
		if len(fields) < 14 {
			continue
		}

		ds, err := c.parse(fields)
		if err != nil {
			continue // parser logs error
		}
		stats[ds.id] = ds
	}

	unitOperationsTag := tags.Tag{Category: "units", Value: "operations"}
	unitBytesTag := tags.Tag{Category: "units", Value: "bytes"}
	unitMillisecondsTag := tags.Tag{Category: "units", Value: "milliseconds"}
	unitSectorsTag := tags.Tag{Category: "units", Value: "sectors"}

	// get list of devices for each entry in mdstats (if it exists)
	mdList := c.parsemdstat()
	mdrx := regexp.MustCompile(`^md[0-9]+`)
	metricType := "L" // uint64
	for devID, devStats := range stats {

		if c.exclude.MatchString(devID) || !c.include.MatchString(devID) {
			c.logger.Debug().Str("device", devID).Msg("excluded device name, ignoring")
			continue
		}

		if mdrx.MatchString(devID) { // is it an md device?
			if devList, ok := mdList[devID]; ok { // have device list for it?
				for _, dev := range devList {
					if ds, found := stats[dev]; found { // have stats for the device?
						// aggregate timings from disks included in raid
						devStats.readms += ds.readms
						devStats.writems += ds.writems
						devStats.currIO += ds.currIO
						devStats.ioms += ds.ioms
						if devStats.haveKernel418 {
							devStats.discardms += ds.discardms
						}
					}
				}
			}
		}

		diskTags := tags.Tags{
			tags.Tag{Category: "device", Value: devID},
		}

		{
			tagList := tags.Tags{unitOperationsTag}
			tagList = append(tagList, diskTags...)
			_ = c.addMetric(&metrics, "", "reads", metricType, devStats.readsCompleted, tagList)
			_ = c.addMetric(&metrics, "", "merged_reads", metricType, devStats.readsMerged, tagList)
			_ = c.addMetric(&metrics, "", "writes", metricType, devStats.writesCompleted, tagList)
			_ = c.addMetric(&metrics, "", "merged_writes", metricType, devStats.writesMerged, tagList)
			_ = c.addMetric(&metrics, "", "iops_in_progress", metricType, devStats.currIO, tagList)
			if devStats.haveKernel418 {
				_ = c.addMetric(&metrics, "", "discards", metricType, devStats.discardsCompleted, tagList)
				_ = c.addMetric(&metrics, "", "merged_discards", metricType, devStats.discardsMerged, tagList)
			}
		}

		{
			tagList := tags.Tags{unitSectorsTag}
			tagList = append(tagList, diskTags...)
			_ = c.addMetric(&metrics, "", "reads", metricType, devStats.sectorsRead, tagList)
			_ = c.addMetric(&metrics, "", "writes", metricType, devStats.sectorsWritten, tagList)
			if devStats.haveKernel418 {
				_ = c.addMetric(&metrics, "", "sectors_discarded", metricType, devStats.sectorsDiscarded, tagList)
			}
		}

		{
			tagList := tags.Tags{unitBytesTag}
			tagList = append(tagList, diskTags...)
			_ = c.addMetric(&metrics, "", "reads", metricType, devStats.bytesRead, tagList)
			_ = c.addMetric(&metrics, "", "writes", metricType, devStats.bytesWritten, tagList)
		}

		{
			tagList := tags.Tags{unitMillisecondsTag}
			tagList = append(tagList, diskTags...)
			_ = c.addMetric(&metrics, "", "read_time", metricType, devStats.readms, tagList)
			_ = c.addMetric(&metrics, "", "write_time", metricType, devStats.writems, tagList)
			_ = c.addMetric(&metrics, "", "io_time", metricType, devStats.ioms, tagList)
			_ = c.addMetric(&metrics, "", "weighted_io_time", metricType, devStats.iomsWeighted, tagList)
			if devStats.haveKernel418 {
				_ = c.addMetric(&metrics, "", "discard_time", metricType, devStats.discardms, tagList)
			}
		}
	}

	c.setStatus(metrics, nil)
	return nil
}

func (c *Disk) getSectorSize(dev string) uint64 {
	if sz, have := c.sectorSizeCache[dev]; have {
		return sz
	}

	sysFSPath := viper.GetString(config.KeyHostSys)
	if sysFSPath == "" {
		sysFSPath = defaults.HostSys
	}
	fn := path.Join(string(os.PathSeparator), sysFSPath, "block", dev, "queue", "physical_block_size")

	c.logger.Debug().Str("fn", fn).Msg("checking for sector size")

	data, err := ioutil.ReadFile(fn)
	if err != nil {
		c.logger.Debug().Err(err).Str("device", dev).Msg("reading block size, using default")
		c.sectorSizeCache[dev] = c.sectorSizeDefault
		return c.sectorSizeDefault
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 32)
	if err != nil {
		c.logger.Debug().Err(err).Str("device", dev).Str("block_size", string(data)).Msg("parsing block size, using default")
		c.sectorSizeCache[dev] = c.sectorSizeDefault
		return c.sectorSizeDefault
	}

	c.sectorSizeCache[dev] = v
	return v
}

func (c *Disk) parse(fields []string) (*dstats, error) {
	devName := fields[2]
	if devName == "" {
		c.logger.Debug().Msg("invalid device name (empty), ignoring")
		return nil, errors.New("invalid device name (empty)")
	}

	sectorSz := c.getSectorSize(devName)

	pe := errors.New("parsing field")
	d := dstats{
		id:            devName,
		haveKernel418: len(fields) > 14,
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
		d.bytesRead = v * sectorSz
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
		d.bytesWritten = v * sectorSz
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

	if d.haveKernel418 {
		if v, err := strconv.ParseUint(fields[14], 10, 64); err == nil {
			d.discardsCompleted = v
		} else {
			c.logger.Warn().Err(err).Str("dev", devName).Msg("parsing field discards completed")
			return nil, pe
		}
		if v, err := strconv.ParseUint(fields[15], 10, 64); err == nil {
			d.discardsMerged = v
		} else {
			c.logger.Warn().Err(err).Str("dev", devName).Msg("parsing field discards merged")
			return nil, pe
		}
		if v, err := strconv.ParseUint(fields[16], 10, 64); err == nil {
			d.sectorsDiscarded = v
		} else {
			c.logger.Warn().Err(err).Str("dev", devName).Msg("parsing field sectors discarded")
			return nil, pe
		}
		if v, err := strconv.ParseUint(fields[17], 10, 64); err == nil {
			d.discardms = v
		} else {
			c.logger.Warn().Err(err).Str("dev", devName).Msg("parsing field discard ms")
			return nil, pe
		}
	}

	return &d, nil
}

func (c *Disk) parsemdstat() map[string][]string {
	mdstatFile := strings.Replace(c.file, c.id, "mdstat", -1)
	mdList := make(map[string][]string)

	mdrx := regexp.MustCompile(`^md[0-9]+`)
	devrx := regexp.MustCompile(`^([^\[]+)\[.+$`)

	lines, err := c.readFile(mdstatFile)
	if err != nil {
		return mdList
	}

	for _, l := range lines {

		line := strings.TrimSpace(string(l))
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}

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

	return mdList
}
