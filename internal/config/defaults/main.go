// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package defaults

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/circonus-labs/circonus-agent/internal/release"
)

const (
	// ListenPort is the default agent tcp listening port
	ListenPort = 2609

	// APIURL for circonus
	APIURL = "https://api.circonus.com/v2/"

	// APIApp defines the api app name associated with the api token key
	APIApp = release.NAME

	// Reverse is false by default
	Reverse = false

	// SSLVerify enabled by default
	SSLVerify = true

	// NoStatsd enabled by default
	NoStatsd = false

	// Debug is false by default
	Debug = false

	// LogLevel set to info by default
	LogLevel = "info"

	// LogPretty colored/formatted output to stderr
	LogPretty = false

	// UID to drop privileges to on start
	UID = "nobody"

	// Watch plugins for changes
	Watch = false

	// StatsdPort to listen, NOTE address is always localhost
	StatsdPort = "8125"

	// StatsdHostPrefix defines that metrics received through StatsD inteface
	// which are prefixed with this string plus a period go to the host check
	StatsdHostPrefix = "host."

	// StatsdHostCategory defines the "plugin" in which the host metrics will be namepspaced
	StatsdHostCategory = "statsd"

	// StatsdGroupPrefix defines that metrics received through StatsD inteface
	// which are prefixed with this string plus a period go to the group check, if enabled
	StatsdGroupPrefix = "group."

	// StatsdGroupCounters defines how group counter metrics will be handled (average or sum)
	StatsdGroupCounters = "sum"

	// StatsdGroupGauges defines how group counter metrics will be handled (average or sum)
	StatsdGroupGauges = "average"

	// StatsdGroupSets defines how group counter metrics will be handled (average or sum)
	StatsdGroupSets = "sum"

	// ReverseCreateCheck flags whether a check, for reverse, should be created if one cannot be found
	ReverseCreateCheck = false

	// ReverseCreateCheckBroker to use if creating a check, 'select' or '' will
	// result in the first broker which meets some basic criteria being selected.
	// 1. Active status
	// 2. Supports the required check type
	// 3. Responds within reverse.brokerMaxResponseTime
	ReverseCreateCheckBroker = "select"

	// ReverseCreateCheckTags to use if creating a check (comma separated list)
	ReverseCreateCheckTags = ""

	// MetricNameSeparator defines character used to delimit metric name parts
	MetricNameSeparator = "`"

	// PluginTTLUnits defines the default TTL units for plugins with TTLs
	// e.g. plugin_ttl30s.sh (30s ttl) plugin_ttl45.sh (would get default ttl units, e.g. 45s)
	PluginTTLUnits = "s" // seconds
)

var (
	// Listen defaults to all interfaces on the default ListenPort
	// valid formats:
	//      ip:port (e.g. 127.0.0.1:12345 - listen address 127.0.0.1, port 12345)
	//      ip (e.g. 127.0.0.1 - listen address 127.0.0.1, port ListenPort)
	//      port (e.g. 12345 (or :12345) - listen address all, port 12345)
	//
	Listen = fmt.Sprintf(":%d", ListenPort)

	// BasePath is the "base" directory
	//
	// expected installation structure:
	// base        (e.g. /opt/circonus/agent)
	//   /bin      (e.g. /opt/circonus/agent/bin)
	//   /etc      (e.g. /opt/circonus/agent/etc)
	//   /plugins  (e.g. /opt/circonus/agent/plugins)
	//   /sbin     (e.g. /opt/circonus/agent/sbin)
	BasePath = ""

	// Collectors defines the default builtin collectors to enable
	// OS specific - see init() below
	Collectors = []string{}

	// EtcPath returns the default etc directory within base directory
	EtcPath = "" // (e.g. /opt/circonus/agent/etc)

	// PluginPath returns the default plugin path
	PluginPath = "" // (e.g. /opt/circonus/agent/plugins)

	// ReverseCreateCheckTitle to use if creating a check
	ReverseCreateCheckTitle = ""

	// ReverseTarget defaults to return from os.Hostname()
	ReverseTarget = ""

	// SSLCertFile returns the deefault ssl cert file name
	SSLCertFile = "" // (e.g. /opt/circonus/agent/etc/agent.pem)

	// SSLKeyFile returns the deefault ssl key file name
	SSLKeyFile = "" // (e.g. /opt/circonus/agent/etc/agent.key)

	// StatsdConf returns the default statsd config file
	StatsdConf = "" // (e.g. /opt/circonus/agent/etc/statsd.json)
)

func init() {
	var exePath string
	var resolvedExePath string
	var err error

	exePath, err = os.Executable()
	if err == nil {
		resolvedExePath, err = filepath.EvalSymlinks(exePath)
		if err == nil {
			BasePath = filepath.Clean(filepath.Join(filepath.Dir(resolvedExePath), "..")) // e.g. /opt/circonus/agent
		}
	}

	if err != nil {
		fmt.Printf("Unable to determine path to binary %v\n", err)
		os.Exit(1)
	}

	EtcPath = filepath.Join(BasePath, "etc")
	PluginPath = filepath.Join(BasePath, "plugins")
	SSLCertFile = filepath.Join(EtcPath, release.NAME+".pem")
	SSLKeyFile = filepath.Join(EtcPath, release.NAME+".key")

	ReverseTarget, err = os.Hostname()
	if err != nil {
		fmt.Printf("Unable to determine hostname for target %v\n", err)
		os.Exit(1)
	}

	ReverseCreateCheckTitle = ReverseTarget + " /agent"

	switch runtime.GOOS {
	case "linux":
		Collectors = []string{
			"cpu",
		}
	case "windows":
		Collectors = []string{
			"cache",
			"disk", // logical and physical
			"interface",
			"ip", // ipv4 and ipv6
			"memory",
			"objects",
			"paging_file",
			// "processes",
			"processor",
			"tcp", // ipv4 and ipv6
			"udp", // ipv4 and ipv6
		}
	}
}
