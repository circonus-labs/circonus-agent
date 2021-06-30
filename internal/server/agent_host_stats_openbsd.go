// Copyright © 2020 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build openbsd

package server

import (
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// agentHostStats produces the internal agent host metrics.
func (s *Server) agentHostStats(metrics cgm.Metrics, mtags []string) {
	///
	// NOTE: host/process packages have an error on openbsd at the moment
	//
	// if ut, err := host.Uptime(); err != nil {
	// 	metrics[tags.MetricNameWithStreamTags("circonus_agent_uptime", tags.FromList(mtags))] = cgm.Metric{Value: ut, Type: "L"}
	// }

	if pcpu, err := cpu.Counts(false); err != nil {
		s.logger.Error().Err(err).Msg("cpu physical")
	} else {
		var ctag []string
		ctag = append(ctag, mtags...)
		ctag = append(ctag, "type:physical")
		metrics[tags.MetricNameWithStreamTags("agent_host_cores", tags.FromList(ctag))] = cgm.Metric{Value: pcpu, Type: "L"}
	}
	if lcpu, err := cpu.Counts(true); err != nil {
		s.logger.Error().Err(err).Msg("cpu logical")
	} else {
		var ctag []string
		ctag = append(ctag, mtags...)
		ctag = append(ctag, "type:logical")
		metrics[tags.MetricNameWithStreamTags("agent_host_cores", tags.FromList(ctag))] = cgm.Metric{Value: lcpu, Type: "L"}
	}

	// memory
	ms, err := mem.VirtualMemory()
	if err != nil {
		s.logger.Error().Err(err).Msg("memory")
	} else {
		var ctag []string
		ctag = append(ctag, mtags...)
		ctag = append(ctag, "units:bytes")
		metrics[tags.MetricNameWithStreamTags("agent_host_memory", tags.FromList(ctag))] = cgm.Metric{Value: ms.Total, Type: "L"}
		// metrics[tags.MetricNameWithStreamTags("agent_host_memory_used", tags.FromList(ctag))] = cgm.Metric{Value: ms.Used, Type: "L"}
	}
}
