// Copyright © 2020 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package server

import (
	"runtime"
	"runtime/debug"

	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
)

// agentStats produces the internal agent metrics
func (s *Server) agentStats(metrics cgm.Metrics, mtags []string) {
	s.agentHostStats(metrics, mtags)

	debug.FreeOSMemory()

	// memory utilization metrics

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	metrics[tags.MetricNameWithStreamTags("agent_mem_frag", tags.FromList(mtags))] = cgm.Metric{Value: float64(ms.Sys-ms.HeapReleased) / float64(ms.HeapInuse), Type: "n"}
	metrics[tags.MetricNameWithStreamTags("agent_numgc", tags.FromList(mtags))] = cgm.Metric{Value: ms.NumGC, Type: "L"}
	metrics[tags.MetricNameWithStreamTags("agent_heap_objs", tags.FromList(mtags))] = cgm.Metric{Value: ms.HeapObjects, Type: "L"}
	metrics[tags.MetricNameWithStreamTags("agent_live_objs", tags.FromList(mtags))] = cgm.Metric{Value: ms.Mallocs - ms.Frees, Type: "L"}

	{
		var ctags []string
		ctags = append(ctags, mtags...)
		ctags = append(ctags, "units:bytes")

		metrics[tags.MetricNameWithStreamTags("agent_heap_alloc", tags.FromList(ctags))] = cgm.Metric{Value: ms.HeapAlloc, Type: "L"}
		metrics[tags.MetricNameWithStreamTags("agent_heap_inuse", tags.FromList(ctags))] = cgm.Metric{Value: ms.HeapInuse, Type: "L"}
		metrics[tags.MetricNameWithStreamTags("agent_heap_idle", tags.FromList(ctags))] = cgm.Metric{Value: ms.HeapIdle, Type: "L"}
		metrics[tags.MetricNameWithStreamTags("agent_heap_released", tags.FromList(ctags))] = cgm.Metric{Value: ms.HeapReleased, Type: "L"}
		metrics[tags.MetricNameWithStreamTags("agent_stack_sys", tags.FromList(ctags))] = cgm.Metric{Value: ms.StackSys, Type: "L"}
		metrics[tags.MetricNameWithStreamTags("agent_other_sys", tags.FromList(ctags))] = cgm.Metric{Value: ms.OtherSys, Type: "L"}

		s.agentMemoryStats(metrics, ctags)
	}
}
