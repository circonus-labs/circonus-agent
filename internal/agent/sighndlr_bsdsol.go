// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build freebsd openbsd solaris darwin

// Signal handling for FreeBSD, OpenBSD, Darwin, and Solaris
// systems that have SIGINFO

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
	signal.Notify(sigch, os.Interrupt, unix.SIGTERM, unix.SIGHUP, unix.SIGPIPE, unix.SIGINFO)
}

// handleSignals runs the signal handler thread
func (a *Agent) handleSignals() {
	const stacktraceBufSize = 1 * units.MiB

	// pre-allocate a buffer
	buf := make([]byte, stacktraceBufSize)

	for {
		select {
		case <-a.shutdownCtx.Done():
			return
		case sig := <-a.signalCh:
			log.Info().Str("signal", sig.String()).Msg("Received signal")
			switch sig {
			case os.Interrupt, unix.SIGTERM:
				a.shutdown()
			case unix.SIGPIPE, unix.SIGHUP:
				// Noop
			case unix.SIGINFO:
				stacklen := runtime.Stack(buf, true)
				fmt.Printf("=== received SIGINFO ===\n*** goroutine dump...\n%s\n*** end\n", buf[:stacklen])
			default:
				panic(fmt.Sprintf("unsupported signal: %v", sig))
			}
		}
	}
}
