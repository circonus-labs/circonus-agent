// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build windows

package agent

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/alecthomas/units"
	"github.com/rs/zerolog/log"
)

func signalNotifySetup(sigch chan os.Signal) {
	signal.Notify(sigch, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE, syscall.SIGTRAP)
}

// handleSignals runs the signal handler thread
func (a *Agent) handleSignals() {
	const stacktraceBufSize = 1 * units.MiB

	// pre-allocate a buffer
	buf := make([]byte, stacktraceBufSize)

	for {
		select {
		case <-a.shutdownCtx.Done():
			log.Debug().Msg("Shutting down")
			return
		case sig := <-a.signalCh:
			log.Info().Str("signal", sig.String()).Msg("Received signal")
			switch sig {
			case os.Interrupt, syscall.SIGTERM:
				a.shutdown()
			case syscall.SIGPIPE, syscall.SIGHUP:
				// Noop
			case syscall.SIGTRAP:
				stacklen := runtime.Stack(buf, true)
				fmt.Printf("=== received SIGINFO ===\n*** goroutine dump...\n%s\n*** end\n", buf[:stacklen])
			default:
				panic(fmt.Sprintf("unsupported signal: %v", sig))
			}
		}
	}
}
