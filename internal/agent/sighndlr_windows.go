// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build windows

// Signal handling for Windows
// doesn't have SIGINFO, attempt to use SIGTRAP instead...

package agent

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/alecthomas/units"
)

func (a *Agent) signalNotifySetup() {
	signal.Notify(a.signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE, syscall.SIGTRAP)
}

// handleSignals runs the signal handler thread
func (a *Agent) handleSignals() error {
	const stacktraceBufSize = 1 * units.MiB

	// pre-allocate a buffer
	buf := make([]byte, stacktraceBufSize)

	for {
		select {
		case sig := <-a.signalCh:
			a.logger.Info().Str("signal", sig.String()).Msg("received signal")
			switch sig {
			case os.Interrupt, syscall.SIGTERM:
				a.Stop()
			case syscall.SIGPIPE, syscall.SIGHUP:
				// Noop
			case syscall.SIGTRAP:
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
