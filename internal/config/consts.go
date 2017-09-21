// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

const (
	cosiName = "cosi"

	// KeyAPICAFile custom ca for circonus api (e.g. inside)
	KeyAPICAFile = "api.ca_file"

	// KeyAPITokenApp circonus api token key application name
	KeyAPITokenApp = "api.app"

	// KeyAPITokenKey circonus api token key
	KeyAPITokenKey = "api.key"

	// KeyAPIURL custom circonus api url (e.g. inside)
	KeyAPIURL = "api.url"

	// KeyDebug enables debug messages
	KeyDebug = "debug"

	// KeyDebugCGM enables debug messages for circonus-gometrics
	KeyDebugCGM = "debug_cgm"

	// KeyListen primary address and port to listen on
	KeyListen = "listen"

	// KeyLogLevel logging level (panic, fatal, error, warn, info, debug, disabled)
	KeyLogLevel = "log.level"

	// KeyLogPretty output formatted log lines (for running in foreground)
	KeyLogPretty = "log.pretty"

	// KeyPluginDir plugin directory
	KeyPluginDir = "plugin-dir"

	// KeyReverse indicates whether to use reverse connections
	KeyReverse = "reverse.enabled"

	// KeyReverseBrokerCAFile custom broker ca file
	KeyReverseBrokerCAFile = "reverse.broker_ca_file"

	// KeyReverseCID circonus check bundle id for reverse
	KeyReverseCID = "reverse.check_bundle_id"

	// KeyReverseTarget custom target|host to use for reverse (searching for a check) default os.Hostname()
	KeyReverseTarget = "reverse.check_target"

	// KeyShowConfig - show configuration and exit
	KeyShowConfig = "show-config"

	// KeyShowVersion - show version information and exit
	KeyShowVersion = "version"

	// KeySSLCertFile pem certificate file for SSL
	KeySSLCertFile = "ssl.cert_file"

	// KeySSLKeyFile key for ssl.cert_file
	KeySSLKeyFile = "ssl.key_file"

	// KeySSLListen ssl address and prot to listen on
	KeySSLListen = "ssl.listen"

	// KeySSLVerify controls verification for ssl connections
	KeySSLVerify = "ssl.verify"

	// KeyStatsdDisabled disables the default statsd listener
	KeyStatsdDisabled = "statsd.disabled"

	// KeyStatsdGroupCID circonus check bundle id for "group" metrics sent to statsd
	KeyStatsdGroupCID = "statsd.group.check_bundle_id"

	// KeyStatsdGroupCounters operator for group counters (sum|average)
	KeyStatsdGroupCounters = "statsd.group.counters"

	// KeyStatsdGroupGauges operator for group gauges (sum|average)
	KeyStatsdGroupGauges = "statsd.group.gauges"

	// KeyStatsdGroupPrefix metrics prefixed with this string are considered "group" metrics
	KeyStatsdGroupPrefix = "statsd.group.metric_prefix"

	// KeyStatsdGroupSets operator for group sets (sum|average)
	KeyStatsdGroupSets = "statsd.group.sets"

	// KeyStatsdHostCategory "plugin" name to put metrics sent to host
	KeyStatsdHostCategory = "statsd.host.category"

	// KeyStatsdHostPrefix metrics prefixed with this string are considered "host" metrics
	KeyStatsdHostPrefix = "statsd.host.metric_prefix"

	// KeyStatsdPort port for statsd listener (note, address will always be 'localhost')
	KeyStatsdPort = "statsd.port"
)
