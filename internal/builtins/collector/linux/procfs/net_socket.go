// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build linux

package procfs

import (
	"fmt"
	"os"
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
)

// NetSocket metrics from the Linux ProcFS
type NetSocket struct {
	common
	include *regexp.Regexp
	exclude *regexp.Regexp
}

// netSocketOptions defines what elements can be overridden in a config file
type netSocketOptions struct {
	// common
	ID         string `json:"id" toml:"id" yaml:"id"`
	ProcFSPath string `json:"procfs_path" toml:"procfs_path" yaml:"procfs_path"`
	RunTTL     string `json:"run_ttl" toml:"run_ttl" yaml:"run_ttl"`

	// collector specific
	IncludeRegex string `json:"include_regex" toml:"include_regex" yaml:"include_regex"`
	ExcludeRegex string `json:"exclude_regex" toml:"exclude_regex" yaml:"exclude_regex"`
}

// NewNetSocketCollector creates new procfs if collector
func NewNetSocketCollector(cfgBaseName, procFSPath string) (collector.Collector, error) {
	procFile := filepath.Join("net", "dev")

	c := NetSocket{
		common: newCommon(NameNetSocket, procFSPath, procFile, tags.FromList(tags.GetBaseTags())),
	}

	c.include = defaultIncludeRegex
	c.exclude = regexp.MustCompile(fmt.Sprintf(regexPat, `lo`))

	if cfgBaseName == "" {
		if _, err := os.Stat(c.file); os.IsNotExist(err) {
			return nil, errors.Wrap(err, c.pkgID)
		}
		return &c, nil
	}

	var opts netSocketOptions
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
func (c *NetSocket) Collect() error {
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

	if err := c.sockstatCollect(&metrics); err != nil {
		c.logger.Warn().Err(err).Msg("sockstat")
	}

	c.setStatus(metrics, nil)
	return nil
}

// type rawSocketStat struct {
// 	name string
// 	val  string
// }

// sockstatCollect gets metrics from /proc/net/sockstat and /proc/net/sockstat6
func (c *NetSocket) sockstatCollect(metrics *cgm.Metrics) error {

	tagUnitsConnections := tags.Tag{Category: "units", Value: "connections"}
	metricType := "l" // int64

	// conns := uint64(0)

	{
		emsg := "sockstat - invalid number of fields"
		sockstatFile := strings.Replace(c.file, "dev", "sockstat", -1)
		lines, err := c.readFile(sockstatFile)
		if err != nil {
			return errors.Wrapf(err, "parsing %s", c.file)
		}

		/*
			sockets: used 176
			TCP: inuse 3 orphan 0 tw 0 alloc 5 mem 1
			UDP: inuse 3 mem 2
			UDPLITE: inuse 0
			RAW: inuse 0
			FRAG: inuse 0 memory 0
		*/

		// stats := make(map[string][]rawSocketStat)

		for _, l := range lines {
			line := strings.TrimSpace(string(l))
			fields := strings.Fields(line)

			statType := strings.ToLower(strings.Replace(fields[0], ":", "", -1))

			switch statType {
			case "sockets":
				if len(fields) != 3 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				name := fields[1]
				if name == "used" {
					val := fields[2]
					v, err := strconv.ParseInt(val, 10, 64)
					if err != nil {
						c.logger.Warn().Str("type", statType).Str("name", name).Str("val", val).Err(err).Msg("sockstat - parsing field")
						break
					}
					tagList := tags.Tags{tags.Tag{Category: "proto", Value: statType}}
					_ = c.addMetric(metrics, "", name, metricType, v, tagList)
				}

			case "tcp":
				if len(fields) != 11 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				{
					name := fields[1]
					if name == "inuse" {
						val := fields[2]
						v, err := strconv.ParseInt(val, 10, 64)
						if err != nil {
							c.logger.Warn().Str("type", statType).Str("name", name).Str("val", val).Err(err).Msg("sockstat - parsing field")
							break
						}
						tagList := tags.Tags{tags.Tag{Category: "proto", Value: statType}, tagUnitsConnections}
						_ = c.addMetric(metrics, "", name, metricType, v, tagList)
					}
				}
				{
					name := fields[3]
					if name == "orphan" {
						val := fields[4]
						v, err := strconv.ParseInt(val, 10, 64)
						if err != nil {
							c.logger.Warn().Str("type", statType).Str("name", name).Str("val", val).Err(err).Msg("sockstat - parsing field")
							break
						}
						tagList := tags.Tags{tags.Tag{Category: "proto", Value: statType}, tagUnitsConnections}
						_ = c.addMetric(metrics, "", name, metricType, v, tagList)
					}
				}
				{
					name := fields[5]
					if name == "tw" {
						// https://github.com/torvalds/linux/blob/master/net/ipv4/proc.c#L50
						// cf. /usr/src/linux/net/ipv4/proc.c
						// tw stands for TIME-WAIT
						// TIME-WAIT - represents waiting for enough time to pass to be sure
						// the remote TCP received the acknowledgment of its connection
						// termination request.
						val := fields[6]
						v, err := strconv.ParseInt(val, 10, 64)
						if err != nil {
							c.logger.Warn().Str("type", statType).Str("name", name).Str("val", val).Err(err).Msg("sockstat - parsing field")
							break
						}
						tagList := tags.Tags{tags.Tag{Category: "proto", Value: statType}, tagUnitsConnections}
						_ = c.addMetric(metrics, "", "time_wait", metricType, v, tagList)
					}
				}
				{
					name := fields[7]
					if name == "alloc" {
						val := fields[8]
						v, err := strconv.ParseInt(val, 10, 64)
						if err != nil {
							c.logger.Warn().Str("type", statType).Str("name", name).Str("val", val).Err(err).Msg("sockstat - parsing field")
							break
						}
						tagList := tags.Tags{tags.Tag{Category: "proto", Value: statType}}
						_ = c.addMetric(metrics, "", "allocated", metricType, v, tagList)
					}
				}
				// stats[statType] = []rawSocketStat{
				// 	{
				// 		name: fields[1], // inuse
				// 		val:  fields[2],
				// 	},
				// 	{
				// 		name: fields[3], // orphan
				// 		val:  fields[4],
				// 	},
				// 	{
				// 		name: fields[5], // tw
				// 		val:  fields[6],
				// 	},
				// 	{
				// 		name: fields[7], // alloc
				// 		val:  fields[8],
				// 	},
				// 	{
				// 		name: fields[9], // mem
				// 		val:  fields[10],
				// 	},
				// }

			case "udp":
				if len(fields) != 5 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				name := fields[1]
				if name == "inuse" {
					val := fields[2]
					v, err := strconv.ParseInt(val, 10, 64)
					if err != nil {
						c.logger.Warn().Str("type", statType).Str("name", name).Str("val", val).Err(err).Msg("sockstat - parsing field")
						break
					}
					tagList := tags.Tags{tags.Tag{Category: "proto", Value: statType}, tagUnitsConnections}
					_ = c.addMetric(metrics, "", name, metricType, v, tagList)
				}
				// stats[statType] = []rawSocketStat{
				// 	{
				// 		name: fields[1], // inuse
				// 		val:  fields[2],
				// 	},
				// 	{
				// 		name: fields[3], // mem
				// 		val:  fields[4],
				// 	},
				// }

			case "udplite":
				if len(fields) != 3 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				name := fields[1]
				if name == "inuse" {
					val := fields[2]
					v, err := strconv.ParseInt(val, 10, 64)
					if err != nil {
						c.logger.Warn().Str("type", statType).Str("name", name).Str("val", val).Err(err).Msg("sockstat - parsing field")
						break
					}
					tagList := tags.Tags{tags.Tag{Category: "proto", Value: statType}, tagUnitsConnections}
					_ = c.addMetric(metrics, "", name, metricType, v, tagList)
				}
				// stats[statType] = []rawSocketStat{
				// 	{
				// 		name: fields[1], // inuse
				// 		val:  fields[2],
				// 	},
				// }

			case "raw":
				if len(fields) != 3 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				name := fields[1]
				if name == "inuse" {
					val := fields[2]
					v, err := strconv.ParseInt(val, 10, 64)
					if err != nil {
						c.logger.Warn().Str("type", statType).Str("name", name).Str("val", val).Err(err).Msg("sockstat - parsing field")
						break
					}
					tagList := tags.Tags{tags.Tag{Category: "proto", Value: statType}, tagUnitsConnections}
					_ = c.addMetric(metrics, "", name, metricType, v, tagList)
				}
				// stats[statType] = []rawSocketStat{
				// 	{
				// 		name: fields[1], // inuse
				// 		val:  fields[2],
				// 	},
				// }

			case "frag":
				if len(fields) != 5 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				name := fields[1]
				if name == "inuse" {
					val := fields[2]
					v, err := strconv.ParseInt(val, 10, 64)
					if err != nil {
						c.logger.Warn().Str("type", statType).Str("name", name).Str("val", val).Err(err).Msg("sockstat - parsing field")
						break
					}
					tagList := tags.Tags{tags.Tag{Category: "proto", Value: statType}, tagUnitsConnections}
					_ = c.addMetric(metrics, "", name, metricType, v, tagList)
				}
				// stats[statType] = []rawSocketStat{
				// 	{
				// 		name: fields[1], // inuse
				// 		val:  fields[2],
				// 	},
				// 	{
				// 		name: fields[3], // mem
				// 		val:  fields[4],
				// 	},
				// }

			default:
				c.logger.Warn().Str("type", statType).Msg("sockstat - unknown stat type, ignoring")

			}
		}

		// for _, n := range stats["tcp"] {
		// 	if n.name == "inuse" {
		// 		v, err := strconv.ParseUint(n.val, 10, 64)
		// 		if err != nil {
		// 			c.logger.Warn().Str("name", n.name).Str("val", n.val).Err(err).Msg("sockstat - parsing field")
		// 			break
		// 		}
		// 		conns += v
		// 		break
		// 	}
		// }
	}

	{
		emsg := "sockstat6 - invalid number of fields"
		sockstatFile := strings.Replace(c.file, "dev", "sockstat6", -1)
		lines, err := c.readFile(sockstatFile)
		if err != nil {
			return errors.Wrapf(err, "parsing %s", c.file)
		}

		/*
		   TCP6: inuse 2
		   UDP6: inuse 2
		   UDPLITE6: inuse 0
		   RAW6: inuse 1
		   FRAG6: inuse 0 memory 0
		*/

		// stats := make(map[string][]rawSocketStat)

		for _, l := range lines {
			line := strings.TrimSpace(string(l))
			fields := strings.Fields(line)

			statType := strings.ToLower(strings.Replace(fields[0], ":", "", -1))

			switch statType {
			case "tcp6":
				if len(fields) != 3 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				name := fields[1]
				if name == "inuse" {
					val := fields[2]
					v, err := strconv.ParseInt(val, 10, 64)
					if err != nil {
						c.logger.Warn().Str("type", statType).Str("name", name).Str("val", val).Err(err).Msg("sockstat - parsing field")
						break
					}
					tagList := tags.Tags{tags.Tag{Category: "proto", Value: statType}, tagUnitsConnections}
					_ = c.addMetric(metrics, "", name, metricType, v, tagList)
				}
				// stats[statType] = []rawSocketStat{
				// 	{
				// 		name: fields[1], // inuse
				// 		val:  fields[2],
				// 	},
				// }

			case "udp6":
				if len(fields) != 3 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				name := fields[1]
				if name == "inuse" {
					val := fields[2]
					v, err := strconv.ParseInt(val, 10, 64)
					if err != nil {
						c.logger.Warn().Str("type", statType).Str("name", name).Str("val", val).Err(err).Msg("sockstat - parsing field")
						break
					}
					tagList := tags.Tags{tags.Tag{Category: "proto", Value: statType}, tagUnitsConnections}
					_ = c.addMetric(metrics, "", name, metricType, v, tagList)
				}
				// stats[statType] = []rawSocketStat{
				// 	{
				// 		name: fields[1], // inuse
				// 		val:  fields[2],
				// 	},
				// }

			case "udplite6":
				if len(fields) != 3 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				name := fields[1]
				if name == "inuse" {
					val := fields[2]
					v, err := strconv.ParseInt(val, 10, 64)
					if err != nil {
						c.logger.Warn().Str("type", statType).Str("name", name).Str("val", val).Err(err).Msg("sockstat - parsing field")
						break
					}
					tagList := tags.Tags{tags.Tag{Category: "proto", Value: statType}, tagUnitsConnections}
					_ = c.addMetric(metrics, "", name, metricType, v, tagList)
				}
				// stats[statType] = []rawSocketStat{
				// 	{
				// 		name: fields[1], // inuse
				// 		val:  fields[2],
				// 	},
				// }

			case "raw6":
				if len(fields) != 3 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				name := fields[1]
				if name == "inuse" {
					val := fields[2]
					v, err := strconv.ParseInt(val, 10, 64)
					if err != nil {
						c.logger.Warn().Str("type", statType).Str("name", name).Str("val", val).Err(err).Msg("sockstat - parsing field")
						break
					}
					tagList := tags.Tags{tags.Tag{Category: "proto", Value: statType}, tagUnitsConnections}
					_ = c.addMetric(metrics, "", name, metricType, v, tagList)
				}
				// stats[statType] = []rawSocketStat{
				// 	{
				// 		name: fields[1], // inuse
				// 		val:  fields[2],
				// 	},
				// }

			case "frag6":
				if len(fields) != 5 {
					c.logger.Warn().Str("type", statType).Msg(emsg)
					continue
				}
				name := fields[1]
				if name == "inuse" {
					val := fields[2]
					v, err := strconv.ParseInt(val, 10, 64)
					if err != nil {
						c.logger.Warn().Str("type", statType).Str("name", name).Str("val", val).Err(err).Msg("sockstat - parsing field")
						break
					}
					tagList := tags.Tags{tags.Tag{Category: "proto", Value: statType}, tagUnitsConnections}
					_ = c.addMetric(metrics, "", name, metricType, v, tagList)
				}
				// stats[statType] = []rawSocketStat{
				// 	{
				// 		name: fields[1], // inuse
				// 		val:  fields[2],
				// 	},
				// 	{
				// 		name: fields[3], // memory
				// 		val:  fields[4],
				// 	},
				// }

			default:
				c.logger.Warn().Str("type", statType).Msg("sockstat6 - unknown stat type, ignoring")

			}
		}

		// for _, n := range stats["tcp6"] {
		// 	if n.name == "inuse" {
		// 		v, err := strconv.ParseUint(n.val, 10, 64)
		// 		if err != nil {
		// 			c.logger.Warn().Err(err).Msg("sockstat6 - parsing tcp6 field " + n.name)
		// 			break
		// 		}
		// 		conns += v
		// 		break
		// 	}
		// }
	}

	// tagUnitsConnections := tags.Tag{Category: "units", Value: "connections"}
	// metricType := "L" // uint64
	// c.addMetric(metrics, "", "socket_connections", metricType, conns, tags.Tags{tagUnitsConnections})

	return nil
}
