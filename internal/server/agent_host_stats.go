// Copyright © 2020 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build !openbsd

package server

import (
	"fmt"
	"os"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/spf13/viper"
)

// agentHostStats produces the internal agent host metrics.
func (s *Server) agentHostStats(metrics cgm.Metrics, mtags []string) {

	// uptime
	if ut, err := host.Uptime(); err != nil {
		s.logger.Error().Err(err).Msg("host uptime")
	} else {
		var ctag []string
		ctag = append(ctag, mtags...)
		ctag = append(ctag, "units:seconds")
		metrics[tags.MetricNameWithStreamTags("agent_host_uptime", tags.FromList(ctag))] = cgm.Metric{Value: ut, Type: "L"}
	}

	// cpus
	{
		if pcpu, err := cpu.Counts(false); err != nil {
			s.logger.Error().Err(err).Msg("physical cores")
		} else {
			var ctag []string
			ctag = append(ctag, mtags...)
			ctag = append(ctag, "type:physical")
			metrics[tags.MetricNameWithStreamTags("agent_host_cores", tags.FromList(ctag))] = cgm.Metric{Value: pcpu, Type: "L"}
		}
		if lcpu, err := cpu.Counts(true); err != nil {
			s.logger.Error().Err(err).Msg("logical cores")
		} else {
			var ctag []string
			ctag = append(ctag, mtags...)
			ctag = append(ctag, "type:logical")
			metrics[tags.MetricNameWithStreamTags("agent_host_cores", tags.FromList(ctag))] = cgm.Metric{Value: lcpu, Type: "L"}
		}
	}
	// memory
	{
		if ms, err := mem.VirtualMemory(); err != nil {
			s.logger.Error().Err(err).Msg("memory")
		} else {
			var ctag []string
			ctag = append(ctag, mtags...)
			ctag = append(ctag, "units:bytes")
			metrics[tags.MetricNameWithStreamTags("agent_host_memory", tags.FromList(ctag))] = cgm.Metric{Value: ms.Total, Type: "L"}
			// metrics[tags.MetricNameWithStreamTags("agent_host_memory_used", tags.FromList(ctag))] = cgm.Metric{Value: ms.Used, Type: "L"}
		}
	}

	// agent process
	{
		pid := os.Getpid()
		if p, err := process.NewProcess(int32(pid)); err != nil {
			s.logger.Error().Err(err).Msg("agent process")
		} else {
			if threads, err := p.NumThreads(); err != nil {
				s.logger.Error().Err(err).Msg("agent process threads")
			} else {
				metrics[tags.MetricNameWithStreamTags("agent_threads", tags.FromList(mtags))] = cgm.Metric{Value: threads, Type: "L"}
			}
		}
	}

	// processes
	{
		memLimit := float32(viper.GetFloat64(config.KeyMemThreshold))
		cpuLimit := viper.GetFloat64(config.KeyCPUThreshold)

		// only do this if someone actually turns it on the default, -1=disabled
		if memLimit < 0 && cpuLimit < 0 {
			return
		}

		pp, err := process.Processes()
		if err != nil {
			s.logger.Error().Err(err).Msg("process list, skipping")
			return
		}

		var merr, cerr, nerr error
		var mp float32
		var cp float64
		var pname string
		for _, p := range pp {
			if running, _ := p.IsRunning(); !running {
				continue
			}

			if memLimit >= 0 {
				mp, merr = p.MemoryPercent()
			}
			if cpuLimit >= 0 {
				cp, cerr = p.CPUPercent()
			}
			pname, nerr = p.Name()

			if merr != nil && cerr != nil {
				continue
			}

			var baseTags []string
			baseTags = append(baseTags, mtags...)
			baseTags = append(baseTags, []string{fmt.Sprintf("pid:%d", p.Pid), "units:percent"}...)
			if nerr == nil {
				baseTags = append(baseTags, "name:"+pname)
			}

			metricName := "process_threshold"

			if merr == nil && memLimit >= 0 {
				if mp >= memLimit {
					var memTags []string
					memTags = append(memTags, baseTags...)
					memTags = append(memTags, "resource:memory")
					metrics[tags.MetricNameWithStreamTags(metricName, tags.FromList(memTags))] = cgm.Metric{Value: mp, Type: "n"}
				}
			}

			if cerr == nil && cpuLimit >= 0 {
				if cp >= cpuLimit {
					var cpuTags []string
					cpuTags = append(cpuTags, baseTags...)
					cpuTags = append(cpuTags, "resource:cpu")
					metrics[tags.MetricNameWithStreamTags(metricName, tags.FromList(cpuTags))] = cgm.Metric{Value: cp, Type: "n"}
				}
			}
		}
	}
}
