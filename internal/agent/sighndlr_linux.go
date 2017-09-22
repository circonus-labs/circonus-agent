// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build linux

// Signal handling for Linux
// system that doesn't have SIGINFO, using SIGUSR1 instead

package agent

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"

	"github.com/alecthomas/units"
	"github.com/rs/zerolog/log"
	"golang.org/x/sys/unix"
)

func signalNotifySetup(sigch chan os.Signal) {
	signal.Notify(sigch, os.Interrupt, unix.SIGTERM, unix.SIGHUP, unix.SIGPIPE, unix.SIGUSR1)
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
			case os.Interrupt, unix.SIGTERM:
				a.shutdown()
			case unix.SIGPIPE, unix.SIGHUP:
				// Noop
			case unix.SIGUSR1:
				stacklen := runtime.Stack(buf, true)
				fmt.Printf("=== received SIGUSR1 ===\n*** goroutine dump...\n%s\n*** end\n", buf[:stacklen])
			default:
				panic(fmt.Sprintf("unsupported signal: %v", sig))
			}
		}
	}
}
