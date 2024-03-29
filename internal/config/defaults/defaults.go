// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package defaults defines default settings for options
package defaults

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/circonus-labs/circonus-agent/internal/release"
)

const (
	HostProc = "/proc"
	HostSys  = "/sys"
	HostEtc  = "/etc"
	HostVar  = "/var"
	HostRun  = "/run"

	// ListenPort is the default agent tcp listening port.
	ListenPort = 2609

	// APIURL for circonus.
	APIURL = "https://api.circonus.com/v2/"

	// APIApp defines the api app name associated with the api token key.
	APIApp = release.NAME

	// Reverse is false by default.
	Reverse = false

	// SSLVerify enabled by default.
	SSLVerify = true

	// NoStatsd enabled by default.
	NoStatsd = false

	// Debug is false by default.
	Debug = false

	// LogLevel set to info by default.
	LogLevel = "info"

	// LogPretty colored/formatted output to stderr.
	LogPretty = false

	// UID to drop privileges to on start.
	UID = "nobody"

	// Watch plugins for changes.
	Watch = false

	// MultiAgent.
	MultiAgent           = false
	MultiAgentInterval   = "60s"
	MultiAgentAccumulate = true

	// ReverseMaxConnRetry - how many times to retry persistently failing broker connection.
	ReverseMaxConnRetry = -1

	// StatsdAddr to listen on.
	StatsdAddr = "localhost"

	// StatsdPort to listen.
	StatsdPort = "8125"

	// StatsdHostPrefix defines that metrics received through StatsD inteface
	// which are prefixed with this string plus a period go to the host check.
	StatsdHostPrefix = "" // "host."

	// StatsdHostCategory defines the "plugin" in which the host metrics will be namepspaced.
	StatsdHostCategory = "statsd"

	// StatsdGroupPrefix defines that metrics received through StatsD inteface
	// which are prefixed with this string plus a period go to the group check, if enabled.
	StatsdGroupPrefix = "group."

	// StatsdGroupCounters defines how group counter metrics will be handled (average or sum).
	StatsdGroupCounters = "sum"

	// StatsdGroupGauges defines how group counter metrics will be handled (average or sum).
	StatsdGroupGauges = "average"

	// StatsdGroupSets defines how group counter metrics will be handled (average or sum).
	StatsdGroupSets = "sum"

	// StatsdEnableTCP defines if the statsd tcp listener is enabled.
	StatsdEnableTCP = false

	// StatsdMaxTCPConns defines the max statsd tcp client connections.
	StatsdMaxTCPConns = uint(250)

	// StatsdNPP sets the number of concurrent packet processors to run.
	StatsdNPP = uint(1)

	// StastdPQS sets the packet queue size.
	StatsdPQS = uint(1000)

	// MetricNameSeparator defines character used to delimit metric name parts.
	MetricNameSeparator = "`"

	// PluginTTLUnits defines the default TTL units for plugins with TTLs
	// e.g. plugin_ttl30s.sh (30s ttl) plugin_ttl45.sh (would get default ttl units, e.g. 45s).
	PluginTTLUnits = "s" // seconds

	// DisableGzip disables gzip compression on responses.
	DisableGzip = false

	// CheckEnableNewMetrics toggles enabling new metrics.
	CheckEnableNewMetrics = false
	// CheckMetricRefreshTTL determines how often to refresh check bundle metrics from API.
	CheckMetricRefreshTTL = "5m"

	// CheckCreate toggles creating a check if a check bundle id is not supplied.
	CheckCreate = false

	// CheckBroker to use if creating a check, 'select' or '' will
	// result in the first broker which meets some basic criteria being selected.
	// 1. Active status
	// 2. Supports the required check type
	// 3. Responds within reverse.brokerMaxResponseTime
	// Enterprise brokers take precedence.
	CheckBroker = "select"

	// CheckTags to use if creating a check (comma separated list).
	CheckTags = ""

	// CheckDelete can delete the created check if the agent had write access to etc/ when it created the check.
	CheckDelete = false

	// Cluster mode enabled.
	ClusterEnabled = false
	// Cluster mode enable builtins (host filesystems must be mounted in container and corresponding
	// `HOST_*`` environment variables must be set).
	ClusterEnableBuiltins = false
	// Cluster mode represent statsd gauges as histogram samples, so that _one_ sample will be collected for each node.
	ClusterStatsdHistogramGauges = false

	// Utilization thresholds for memory and cpu to trigger sending process metric (if a given process is higher for EITHER)
	// -1 disables the feature.
	CPUThreshold = float32(-1)
	MemThreshold = float32(-1)
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
	// OS specific - see init() below.
	Collectors = []string{}

	// EtcPath returns the default etc directory within base directory.
	EtcPath = "" // (e.g. /opt/circonus/agent/etc)

	// PluginPath returns the default plugin path.
	PluginPath = "" // (e.g. /opt/circonus/agent/plugins)

	// CheckTarget defaults to return from os.Hostname().
	CheckTarget = ""

	// CheckTitle defaults to '<CheckTarget> /agent'.
	CheckTitle = ""

	// CheckMetricStatePath returns the default state directory. In order for
	// automatic new metric enabling to work the state path must exist
	// and be owned by the user running circonus-agentd (i.e. 'nobody').
	CheckMetricStatePath = "" // (e.g. /opt/circonus/agent/state)

	// CheckMetricFilters defines default filter to be used with new check creation.
	CheckMetricFilters = [][]string{{"deny", "^$", ""}, {"allow", "^.+$", ""}}
	// CheckMetricFilterFile defines an external file (json) with metric filter definitions.
	CheckMetricFilterFile = ""
	// CheckUpdateMetricFilters will overwrite filters on the check every time the agent starts
	// with whatever rules are in the agent configuration (or external metric filters file).
	CheckUpdateMetricFilters = false

	// CheckPeriod how often broker requests metrics.
	CheckPeriod = uint(60)
	// CheckTimeout broker timeout when requesting metrics.
	CheckTimeout = float64(10)

	// CheckUpdate forces updating all configurable check attributes.
	CheckUpdate = false

	// SSLCertFile returns the deefault ssl cert file name.
	SSLCertFile = "" // (e.g. /opt/circonus/agent/etc/agent.pem)

	// SSLKeyFile returns the deefault ssl key file name.
	SSLKeyFile = "" // (e.g. /opt/circonus/agent/etc/agent.key)

	// StatsdConf returns the default statsd config file.
	StatsdConf = "" // (e.g. /opt/circonus/agent/etc/statsd.json)

	CheckType = ""

	CheckBundleFile = ""
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
	CheckMetricStatePath = filepath.Join(BasePath, "state")
	PluginPath = filepath.Join(BasePath, "plugins")
	SSLCertFile = filepath.Join(EtcPath, release.NAME+".pem")
	SSLKeyFile = filepath.Join(EtcPath, release.NAME+".key")
	CheckMetricFilterFile = filepath.Join(EtcPath, "metric_filters.json")
	CheckBundleFile = filepath.Join(EtcPath, "check_bundle.json")

	CheckTarget, err = os.Hostname()
	if err != nil {
		fmt.Printf("Unable to determine hostname for target %v\n", err)
		os.Exit(1)
	}
	CheckTitle = CheckTarget + " /agent"

	CheckType = "json:" + release.NAME + ":" + runtime.GOOS

	switch runtime.GOOS {
	case "linux":
		Collectors = []string{
			"procfs/cpu",
			"procfs/disk",
			"generic/fs",
			"procfs/if",
			"procfs/load",
			"procfs/proto",
			"procfs/vm",
		}
	case "windows":
		Collectors = []string{
			"wmi/cache",
			"wmi/disk", // logical and physical
			"wmi/interface",
			"wmi/ip", // ipv4 and ipv6
			"wmi/memory",
			"wmi/objects",
			"wmi/paging_file",
			// "wmi/processes",
			"wmi/processor",
			"wmi/tcp", // ipv4 and ipv6
			"wmi/udp", // ipv4 and ipv6
			"wmi/system",
		}
	case "darwin":
		Collectors = []string{
			// "generic/cpu",
			// "generic/disk",
			"generic/fs",
			"generic/if",
			"generic/load",
			// "generic/proto",
			"generic/vm",
		}
	default:
		Collectors = []string{
			"generic/cpu",
			"generic/disk",
			"generic/fs",
			"generic/if",
			"generic/load",
			"generic/proto",
			"generic/vm",
		}
	}
}
