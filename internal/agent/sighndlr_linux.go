// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

//go:build linux
// +build linux

// Signal handling for Linux
// doesn't have SIGINFO, using SIGTRAP instead

package agent

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"

	"github.com/alecthomas/units"
	"golang.org/x/sys/unix"
)

func (a *Agent) signalNotifySetup() {
	signal.Notify(a.signalCh, os.Interrupt, unix.SIGTERM, unix.SIGHUP, unix.SIGTRAP)
}

// handleSignals runs the signal handler thread.
func (a *Agent) handleSignals() error {
	const stacktraceBufSize = 1 * units.MiB

	// pre-allocate a buffer
	buf := make([]byte, stacktraceBufSize)

	for {
		select {
		case sig := <-a.signalCh:
			a.logger.Info().Str("signal", sig.String()).Msg("received signal")
			switch sig {
			case os.Interrupt, unix.SIGTERM:
				a.Stop()
			case unix.SIGHUP:
				// Noop
			case unix.SIGTRAP:
				stacklen := runtime.Stack(buf, true)
				fmt.Printf("=== received SIGTRAP ===\n*** goroutine dump...\n%s\n*** end\n", buf[:stacklen])
			default:
				a.logger.Warn().Str("signal", sig.String()).Msg("unsupported")
			}
		case <-a.groupCtx.Done():
			return nil
		}
	}
}
