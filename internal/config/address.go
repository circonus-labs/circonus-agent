// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

// ParseListen parses and fixes listen spec.
import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
)

// ParseListen verifies and parses a listen address spec.
func ParseListen(spec string) (*net.TCPAddr, error) {
	// empty, default
	if spec == "" {
		spec = defaults.Listen
	}
	// only a port, prefix with colon
	if ok, _ := regexp.MatchString(`^[0-9]+$`, spec); ok {
		spec = ":" + spec
	}
	// ipv4 w/o port, add default
	if strings.Contains(spec, ".") && !strings.Contains(spec, ":") {
		spec += defaults.Listen
	}
	// ipv6 w/o port, add default
	if ok, _ := regexp.MatchString(`^\[[a-f0-9:]+\]$`, spec); ok {
		spec += defaults.Listen
	}

	host, port, err := net.SplitHostPort(spec)
	if err != nil {
		return nil, fmt.Errorf("parsing listen: %w", err)
	}

	addr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(host, port))
	if err != nil {
		return nil, fmt.Errorf("resolving listen: %w", err)
	}

	return addr, nil
}
