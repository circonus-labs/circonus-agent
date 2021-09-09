// Copyright © 2020 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

//go:build solaris || illumos
// +build solaris illumos

package server

import (
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
)

// agentMemoryStats produces the internal agent metrics.
func (s *Server) agentMemoryStats(metrics cgm.Metrics, mtags []string) {
	// var mem syscall.Rusage
	// if err := syscall.Getrusage(syscall.RUSAGE_SELF, &mem); err == nil {
	// 	metrics[tags.MetricNameWithStreamTags("agent_max_rss", tags.FromList(ctags))] = cgm.Metric{Value: uint64(mem.Maxrss * 1024), Type: "L"} // maximum resident set size used (in kilobytes)
	// } else {
	// 	s.logger.Warn().Err(err).Msg("collecting rss from system")
	// }
}
