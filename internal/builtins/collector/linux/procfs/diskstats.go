// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build linux

package procfs

import (
	"bufio"
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
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// Diskstats metrics from the Linux ProcFS
type Diskstats struct {
	pfscommon
	include           *regexp.Regexp
	exclude           *regexp.Regexp
	sectorSizeDefault uint64
	sectorSizeCache   map[string]uint64
}

// diskstatsOptions defines what elements can be overridden in a config file
type diskstatsOptions struct {
	// common
	ID                   string   `json:"id" toml:"id" yaml:"id"`
	ProcFSPath           string   `json:"procfs_path" toml:"procfs_path" yaml:"procfs_path"`
	MetricsEnabled       []string `json:"metrics_enabled" toml:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsDisabled      []string `json:"metrics_disabled" toml:"metrics_disabled" yaml:"metrics_disabled"`
	MetricsDefaultStatus string   `json:"metrics_default_status" toml:"metrics_default_status" toml:"metrics_default_status"`
	RunTTL               string   `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`

	// collector specific
	IncludeRegex      string `json:"include_regex" toml:"include_regex" yaml:"include_regex"`
	ExcludeRegex      string `json:"exclude_regex" toml:"exclude_regex" yaml:"exclude_regex"`
	DefaultSectorSize string `json:"default_sector_size" toml:"default_sector_size" yaml:"default_sector_size"`
}

type dstats struct {
	id              string
	readsCompleted  uint64
	readsMerged     uint64
	sectorsRead     uint64
	bytesRead       uint64
	readms          uint64
	writesCompleted uint64
	writesMerged    uint64
	sectorsWritten  uint64
	bytesWritten    uint64
	writems         uint64
	currIO          uint64
	ioms            uint64
	iomsWeighted    uint64
}

// NewDiskstatsCollector creates new procfs diskstats collector
func NewDiskstatsCollector(cfgBaseName, procFSPath string) (collector.Collector, error) {
	procFile := DISKSTATS_NAME

	c := Diskstats{}
	c.id = DISKSTATS_NAME
	c.pkgID = PFS_PREFIX + c.id
	c.procFSPath = procFSPath
	c.file = filepath.Join(c.procFSPath, procFile)
	c.logger = log.With().Str("pkg", c.pkgID).Logger()
	c.metricStatus = map[string]bool{}
	c.metricDefaultActive = true
	c.sectorSizeCache = make(map[string]uint64)
	c.baseTags = tags.FromList(tags.GetBaseTags())

	c.include = defaultIncludeRegex
	c.exclude = defaultExcludeRegex
	c.sectorSizeDefault = 512

	if cfgBaseName == "" {
		if _, err := os.Stat(c.file); os.IsNotExist(err) {
			return nil, errors.Wrap(err, c.pkgID)
		}
		return &c, nil
	}

	var opts diskstatsOptions
	err := config.LoadConfigFile(cfgBaseName, &opts)
	if err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			return &c, nil
		}
		c.logger.Warn().Err(err).Str("file", cfgBaseName).Msg("loading config file")
		return nil, errors.Wrapf(err, "%s config", c.pkgID)
	}

	c.logger.Debug().Str("base", cfgBaseName).Interface("config", opts).Msg("loaded config")

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

	if len(opts.MetricsEnabled) > 0 {
		for _, name := range opts.MetricsEnabled {
			c.metricStatus[name] = true
		}
	}
	if len(opts.MetricsDisabled) > 0 {
		for _, name := range opts.MetricsDisabled {
			c.metricStatus[name] = false
		}
	}

	if opts.MetricsDefaultStatus != "" {
		if ok, _ := regexp.MatchString(`^(enabled|disabled)$`, strings.ToLower(opts.MetricsDefaultStatus)); ok {
			c.metricDefaultActive = strings.ToLower(opts.MetricsDefaultStatus) == metricStatusEnabled
		} else {
			return nil, errors.Errorf("%s invalid metric default status (%s)", c.pkgID, opts.MetricsDefaultStatus)
		}
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
func (c *Diskstats) Collect() error {
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
		return errors.Wrap(err, c.pkgID)
	}
	defer f.Close()

	stats := make(map[string]*dstats)
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
		return errors.Wrapf(err, "%s parsing %s", c.pkgID, f.Name())
	}

	// get list of devices for each entry in mdstats (if it exists)
	mdList := c.parsemdstat()
	mdrx := regexp.MustCompile(`^md[0-9]+`)
	pfx := c.id + metricNameSeparator
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
					}
				}
			}
		}

		c.addMetric(&metrics, pfx+devID, "rd_completed", metricType, devStats.readsCompleted)
		c.addMetric(&metrics, pfx+devID, "rd_merged", metricType, devStats.readsMerged)
		c.addMetric(&metrics, pfx+devID, "rd_sectors", metricType, devStats.sectorsRead)
		c.addMetric(&metrics, pfx+devID, "rd_bytes", metricType, devStats.bytesRead)
		c.addMetric(&metrics, pfx+devID, "rd_ms", metricType, devStats.readms)
		c.addMetric(&metrics, pfx+devID, "wr_completed", metricType, devStats.writesCompleted)
		c.addMetric(&metrics, pfx+devID, "wr_merged", metricType, devStats.writesMerged)
		c.addMetric(&metrics, pfx+devID, "wr_sectors", metricType, devStats.sectorsWritten)
		c.addMetric(&metrics, pfx+devID, "wr_bytes", metricType, devStats.bytesWritten)
		c.addMetric(&metrics, pfx+devID, "wr_ms", metricType, devStats.writems)
		c.addMetric(&metrics, pfx+devID, "io_in_progress", metricType, devStats.currIO)
		c.addMetric(&metrics, pfx+devID, "io_ms", metricType, devStats.ioms)
		c.addMetric(&metrics, pfx+devID, "io_ms_weighted", metricType, devStats.iomsWeighted)
	}

	c.setStatus(metrics, nil)
	return nil
}

func (c *Diskstats) getSectorSize(dev string) uint64 {
	if sz, have := c.sectorSizeCache[dev]; have {
		return sz
	}

	fn := path.Join(string(os.PathSeparator), "sys", "block", dev, "queue", "physical_block_size")

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

func (c *Diskstats) parse(fields []string) (*dstats, error) {
	devName := fields[2]
	if devName == "" {
		c.logger.Debug().Msg("invalid device name (empty), ignoring")
		return nil, errors.New("invalid device name (empty)")
	}

	sectorSz := c.getSectorSize(devName)

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

	return &d, nil
}

func (c *Diskstats) parsemdstat() map[string][]string {
	mdstatPath := strings.Replace(c.file, DISKSTATS_NAME, "mdstat", -1)
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

	if err := scanner.Err(); err != nil {
		c.logger.Debug().Err(err).Str("mdstats", mdstatPath).Msg("loading mdstats, ignoring")
		return map[string][]string{}
	}
	return mdList
}
