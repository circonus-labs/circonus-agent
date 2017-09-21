// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"net"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

func parseListen(spec, defaultSpec string) (string, string, error) {
	if spec == "" && defaultSpec == "" {
		return "", "", nil
	}

	// fixup the default spec for parsing
	if defaultSpec != "" {
		if !strings.Contains(defaultSpec, ":") {
			if strings.Contains(defaultSpec, ".") {
				defaultSpec += ":" // e.g. 127.0.0.1 -> 127.0.0.1:
			} else {
				defaultSpec = ":" + defaultSpec // e.g. 1234 -> :1234
			}
		}
	}
	defaultIP, defaultPort, _ := net.SplitHostPort(defaultSpec)

	// fixup the custom spec for parsing
	if spec != "" {
		if !strings.Contains(spec, ":") {
			if strings.Contains(spec, ".") {
				spec += ":"
			} else {
				spec = ":" + spec
			}
		}
	}
	ip, port, _ := net.SplitHostPort(spec)

	if ip == "" {
		ip = defaultIP
	}

	if port == "" {
		port = defaultPort
	}

	if ip == "" && port == "" {
		return "", "", errors.Errorf("Missing IP (%s) and Port (%s) in specification (%s)", ip, port, spec)
	}

	if ip != "" && net.ParseIP(ip) == nil {
		return "", "", errors.Errorf("Invalid IP address format specified '%s'", ip)
	}

	if port != "" {
		uport, err := strconv.Atoi(port)
		if err != nil {
			return "", "", errors.Wrap(err, "Invalid port")
		}
		if uport <= 0 || uport >= 65535 {
			return "", "", errors.Errorf("Invalid port, out of range 0<%s<65535", port)
		}
	}

	return ip, port, nil
}
