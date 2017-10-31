// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"path/filepath"

	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
)

// Log defines the running config.log structure
type Log struct {
	Level  string `json:"level" yaml:"level" toml:"level"`
	Pretty bool   `json:"pretty" yaml:"pretty" toml:"pretty"`
}

// API defines the running config.api structure
type API struct {
	Key    string `json:"key" yaml:"key" toml:"key"`
	App    string `json:"app" yaml:"app" toml:"app"`
	URL    string `json:"url" yaml:"url" toml:"url"`
	CAFile string `mapstructure:"ca_file" json:"ca_file" yaml:"ca_file" toml:"ca_file"`
}

// ReverseCreateCheckOptions defines the running config.reverse.check structure
type ReverseCreateCheckOptions struct {
	Broker string `json:"broker" yaml:"broker" toml:"broker"`
	Tags   string `json:"tags" yaml:"tags" toml:"tags"`
	Title  string `json:"title" yaml:"title" toml:"title"`
}

// Reverse defines the running config.reverse structure
type Reverse struct {
	Enabled            bool                      `json:"enabled" yaml:"enabled" toml:"enabled"`
	BrokerCAFile       string                    `mapstructure:"broker_ca_file" json:"broker_ca_file" yaml:"broker_ca_file" toml:"broker_ca_file"`
	CheckBundleID      string                    `mapstructure:"check_bundle_id" json:"check_bundle_id" yaml:"check_bundle_id" toml:"check_bundle_id"`
	CheckTarget        string                    `mapstructure:"check_target" json:"check_target" yaml:"check_target" toml:"check_target"`
	CreateCheck        bool                      `mapstructure:"create_check" json:"create_check" yaml:"create_check" toml:"create_check"`
	CreateCheckOptions ReverseCreateCheckOptions `json:"check" yaml:"check" toml:"check"`
}

// SSL defines the running config.ssl structure
type SSL struct {
	CertFile string `mapstructure:"cert_file" json:"cert_file" yaml:"cert_file" toml:"cert_file"`
	KeyFile  string `mapstructure:"key_file" json:"key_file" yaml:"key_file" toml:"key_file"`
	Listen   string `json:"listen" yaml:"listen" toml:"listen"`
	Verify   bool   `json:"verify" yaml:"verify" toml:"verify"`
}

// StatsDHost defines the running config.statsd.host structure
type StatsDHost struct {
	Category     string `json:"category" yaml:"category" toml:"category"`
	MetricPrefix string `mapstructure:"metric_prefix" json:"metric_prefix" yaml:"metric_prefix" toml:"metric_prefix"`
}

// StatsDGroup defines the running config.statsd.group structure
type StatsDGroup struct {
	CheckBundleID string `mapstructure:"check_bundle_id" json:"check_bundle_id" yaml:"check_bundle_id" toml:"check_bundle_id"`
	Counters      string `json:"counters" yaml:"counters" toml:"counters"`
	Gauges        string `json:"gauges" yaml:"gauges" toml:"gauges"`
	MetricPrefix  string `mapstructure:"metric_prefix" json:"metric_prefix" yaml:"metric_prefix" toml:"metric_prefix"`
	Sets          string `json:"sets" yaml:"sets" toml:"sets"`
}

// StatsD defines the running config.statsd structure
type StatsD struct {
	Disabled bool        `json:"disabled" yaml:"disabled" toml:"disabled"`
	Port     string      `json:"port" yaml:"port" toml:"port"`
	Group    StatsDGroup `json:"group" yaml:"group" toml:"group"`
	Host     StatsDHost  `json:"host" yaml:"host" toml:"host"`
}

// Config defines the running config structure
type Config struct {
	Debug          bool     `json:"debug" yaml:"debug" toml:"debug"`
	API            API      `json:"api" yaml:"api" toml:"api"`
	Log            Log      `json:"log" yaml:"log" toml:"log"`
	DebugCGM       bool     `mapstructure:"debug_cgm" json:"debug_cgm" yaml:"debug_cgm" toml:"debug_cgm"`
	Listen         []string `json:"listen" yaml:"listen" toml:"listen"`
	ListenSocket   []string `mapstructure:"listen_socket" json:"listen_socket" yaml:"listen_socket" toml:"listen_socket"`
	PluginDir      string   `mapstructure:"plugin_dir" json:"plugin_dir" yaml:"plugin_dir" toml:"plugin_dir"`
	PluginTTLUnits string   `mapstructure:"plugin_ttl_units" json:"plugin_ttl_units" yaml:"plugin_ttl_units" toml:"plugin_ttl_units"`
	Reverse        Reverse  `json:"reverse" yaml:"reverse" toml:"reverse"`
	SSL            SSL      `json:"ssl" yaml:"ssl" toml:"ssl"`
	StatsD         StatsD   `json:"statsd" yaml:"statsd" toml:"statsd"`
	Collectors     []string `json:"collectors" yaml:"collectors" toml:"collectors"`
}

type cosiCheckConfig struct {
	CID string `json:"_cid"`
}

//
// NOTE: adding a Key* MUST be reflected in the Config structures above
//
const (
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

	// KeyListenSocket identifies one or more unix socket files to create
	KeyListenSocket = "listen_socket"

	// KeyLogLevel logging level (panic, fatal, error, warn, info, debug, disabled)
	KeyLogLevel = "log.level"

	// KeyLogPretty output formatted log lines (for running in foreground)
	KeyLogPretty = "log.pretty"

	// KeyPluginDir plugin directory
	KeyPluginDir = "plugin_dir"

	// KeyPluginTTLUnits plugin run ttl units
	KeyPluginTTLUnits = "plugin_ttl_units"

	// KeyReverse indicates whether to use reverse connections
	KeyReverse = "reverse.enabled"

	// KeyReverseBrokerCAFile custom broker ca file
	KeyReverseBrokerCAFile = "reverse.broker_ca_file"

	// KeyReverseCID circonus check bundle id for reverse
	KeyReverseCID = "reverse.check_bundle_id"

	// KeyReverseTarget custom target|host to use for reverse (searching for a check) default os.Hostname()
	KeyReverseTarget = "reverse.check_target"

	// KeyReverseCreateCheck indicates whether a check should be created if one cannot be found for the target
	KeyReverseCreateCheck = "reverse.create_check"

	// KeyReverseCreateCheckBroker cid to use if creating a check
	KeyReverseCreateCheckBroker = "reverse.check.broker"

	// KeyReverseCreateCheckTitle to use if creating a check
	KeyReverseCreateCheckTitle = "reverse.check.title"

	// KeyReverseCreateCheckTags to add if creating a check
	KeyReverseCreateCheckTags = "reverse.check.tags"

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

	// KeyCollectors defines the builtin collectors to enable
	KeyCollectors = "collectors"

	cosiName = "cosi"
)

var (
	cosiCfgFile = filepath.Join(defaults.BasePath, "..", cosiName, "etc", "cosi.json")

	// MetricNameSeparator defines character used to delimit metric name parts
	MetricNameSeparator = defaults.MetricNameSeparator // var, TBD whether it will become configurable
)
