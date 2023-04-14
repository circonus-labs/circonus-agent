// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package config defines options
package config

import (
	"encoding/json"
	"expvar"
	"fmt"
	"io"

	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	toml "github.com/pelletier/go-toml"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
)

// Log defines the running config.log structure.
type Log struct {
	Level  string `json:"level" yaml:"level" toml:"level"`
	Pretty bool   `json:"pretty" yaml:"pretty" toml:"pretty"`
}

// API defines the running config.api structure.
type API struct {
	App    string `json:"app" yaml:"app" toml:"app"`
	CAFile string `mapstructure:"ca_file" json:"ca_file" yaml:"ca_file" toml:"ca_file"`
	Key    string `json:"key" yaml:"key" toml:"key"`
	URL    string `json:"url" yaml:"url" toml:"url"`
}

// ReverseCreateCheckOptions defines the running config.reverse.check structure.
type ReverseCreateCheckOptions struct {
	Broker string `json:"broker" yaml:"broker" toml:"broker"`
	Tags   string `json:"tags" yaml:"tags" toml:"tags"`
	Title  string `json:"title" yaml:"title" toml:"title"`
}

// Check defines the check parameters.
type Check struct {
	Broker           string `json:"broker" yaml:"broker" toml:"broker"`
	BundleID         string `mapstructure:"bundle_id" json:"bundle_id" yaml:"bundle_id" toml:"bundle_id"`
	MetricFilterFile string `mapstructure:"metric_filter_file" json:"metric_filter_file" yaml:"metric_filter_file" toml:"metric_filter_file"`
	MetricFilters    string `mapstructure:"metric_filters" json:"metric_filters" yaml:"metric_filters" toml:"metric_filters"` // needs to be json embedded in a string because rules are positional
	Tags             string `json:"tags" yaml:"tags" toml:"tags"`
	Target           string `mapstructure:"target" json:"target" yaml:"target" toml:"target"`
	Title            string `json:"title" yaml:"title" toml:"title"`
	// MetricRefreshTTL    string  `json:"-" yaml:"-" toml:"-"` // deprecated, hidden
	// MetricStateDir      string  `json:"-" yaml:"-" toml:"-"` // deprecated, hidden
	Period              uint    `json:"period" toml:"period" yaml:"period"`
	Timeout             float64 `json:"timeout" toml:"timeout" yaml:"timeout"`
	Create              bool    `mapstructure:"create" json:"create" yaml:"create" toml:"create"`
	MetricStreamtags    bool    `mapstructure:"metric_streamtags" json:"metric_streamtags" yaml:"metric_streamtags" toml:"metric_streamtags"`
	Update              bool    `json:"update" toml:"update" yaml:"update"`
	UpdateMetricFilters bool    `mapstructure:"update_metric_filters" json:"update_metric_filters" yaml:"update_metric_filters" toml:"update_metric_filters"`
	// EnableNewMetrics    bool    `json:"-" yaml:"-" toml:"-"` // deprecated, hidden
}

// MultiAgent defines the running config.multi_agent structure.
type MultiAgent struct {
	Interval   string `json:"interval" toml:"interval" yaml:"interval"`
	Enabled    bool   `json:"enabled" toml:"enabled" yaml:"enabled"`
	Accumulate bool   `json:"accumulate" toml:"accumulate" yaml:"accumulate"`
}

// Reverse defines the running config.reverse structure.
type Reverse struct {
	BrokerCAFile string `mapstructure:"broker_ca_file" json:"broker_ca_file" yaml:"broker_ca_file" toml:"broker_ca_file"`
	Enabled      bool   `json:"enabled" yaml:"enabled" toml:"enabled"`
	MaxConnRetry int    `mapstructure:"max_conn_retry" json:"max_conn_retry" yaml:"max_conn_retry" toml:"max_conn_retry"`
}

// SSL defines the running config.ssl structure.
type SSL struct {
	CertFile string `mapstructure:"cert_file" json:"cert_file" yaml:"cert_file" toml:"cert_file"`
	KeyFile  string `mapstructure:"key_file" json:"key_file" yaml:"key_file" toml:"key_file"`
	Listen   string `json:"listen" yaml:"listen" toml:"listen"`
	Verify   bool   `json:"verify" yaml:"verify" toml:"verify"`
}

// StatsDHost defines the running config.statsd.host structure.
type StatsDHost struct {
	Category     string `json:"category" yaml:"category" toml:"category"`
	MetricPrefix string `mapstructure:"metric_prefix" json:"metric_prefix" yaml:"metric_prefix" toml:"metric_prefix"`
}

// StatsDGroup defines the running config.statsd.group structure.
type StatsDGroup struct {
	CheckID      string `mapstructure:"check_id" json:"check_id" yaml:"check_id" toml:"check_id"`
	Counters     string `json:"counters" yaml:"counters" toml:"counters"`
	Gauges       string `json:"gauges" yaml:"gauges" toml:"gauges"`
	MetricPrefix string `mapstructure:"metric_prefix" json:"metric_prefix" yaml:"metric_prefix" toml:"metric_prefix"`
	Sets         string `json:"sets" yaml:"sets" toml:"sets"`
}

// StatsD defines the running config.statsd structure.
type StatsD struct {
	Group              StatsDGroup `json:"group" yaml:"group" toml:"group"`
	Host               StatsDHost  `json:"host" yaml:"host" toml:"host"`
	Addr               string      `join:"addr" yaml:"addr" toml:"addr"`
	Port               string      `json:"port" yaml:"port" toml:"port"`
	NPP                uint        `json:"npp" yaml:"npp" toml:"npp"`
	PQS                uint        `json:"pqs" yaml:"pqs" toml:"pqs"`
	Disabled           bool        `json:"disabled" yaml:"disabled" toml:"disabled"`
	DebugMetricParsing bool        `mapstructure:"debug_metric_parsing" json:"debug_metric_parsing" toml:"debug_metrics_parsing" yaml:"debug_metric_parsing"`
}

// Thresholds defines triggers used to include metrics.
type Thresholds struct {
	HighCPU    float32 `mapstructure:"high_cpu" json:"high_cpu" toml:"high_cpu" yaml:"high_cpu"`
	HighMemory float32 `mapstructure:"high_memory" json:"high_memory" toml:"high_memory" yaml:"high_memory"`
}

// Config defines the running config structure.
type Config struct {
	DebugDumpMetrics string     `mapstructure:"debug_dump_metrics" json:"debug_dump_metrics" yaml:"debug_dump_metrics" toml:"debug_dump_metrics"`
	PluginDir        string     `mapstructure:"plugin_dir" json:"plugin_dir" yaml:"plugin_dir" toml:"plugin_dir"`
	PluginTTLUnits   string     `mapstructure:"plugin_ttl_units" json:"plugin_ttl_units" yaml:"plugin_ttl_units" toml:"plugin_ttl_units"`
	HostProc         string     `mapstructure:"host_proc" json:"host_proc" toml:"host_proc" yaml:"host_proc"`
	HostSys          string     `mapstructure:"host_sys" json:"host_sys" toml:"host_sys" yaml:"host_sys"`
	HostEtc          string     `mapstructure:"host_etc" json:"host_etc" toml:"host_etc" yaml:"host_etc"`
	HostVar          string     `mapstructure:"host_var" json:"host_var" toml:"host_var" yaml:"host_var"`
	HostRun          string     `mapstructure:"host_run" json:"host_run" toml:"host_run" yaml:"host_run"`
	API              API        `json:"api" yaml:"api" toml:"api"`
	SSL              SSL        `json:"ssl" yaml:"ssl" toml:"ssl"`
	Collectors       []string   `json:"collectors" yaml:"collectors" toml:"collectors"`
	Listen           []string   `json:"listen" yaml:"listen" toml:"listen"`
	ListenSocket     []string   `mapstructure:"listen_socket" json:"listen_socket" yaml:"listen_socket" toml:"listen_socket"`
	PluginList       []string   `mapstructure:"plugin_list" json:"plugin_list" yaml:"plugin_list" toml:"plugin_list"`
	Log              Log        `json:"log" yaml:"log" toml:"log"`
	StatsD           StatsD     `json:"statsd" yaml:"statsd" toml:"statsd"`
	MultiAgent       MultiAgent `mapstructure:"multi_agent" json:"multi_agent" toml:"multi_agent" yaml:"multi_agent"`
	Reverse          Reverse    `json:"reverse" yaml:"reverse" toml:"reverse"`
	Check            Check      `json:"check" yaml:"check" toml:"check"`
	Thresholds       Thresholds `mapstructure:"thresholds" json:"thresholds" toml:"thresholds" yaml:"thresholds"`
	Debug            bool       `json:"debug" yaml:"debug" toml:"debug"`
	DebugCGM         bool       `mapstructure:"debug_cgm" json:"debug_cgm" yaml:"debug_cgm" toml:"debug_cgm"`
	DebugAPI         bool       `mapstructure:"debug_api" json:"debug_api" yaml:"debug_api" toml:"debug_api"`
}

// NOTE: adding a Key* MUST be reflected in the Config structures above.
const (
	// KeyAPICAFile custom ca for circonus api (e.g. inside).
	KeyAPICAFile = "api.ca_file"

	// KeyAPITokenApp circonus api token key application name.
	KeyAPITokenApp = "api.app"

	// KeyAPITokenKey circonus api token key.
	KeyAPITokenKey = "api.key"

	// KeyAPIURL custom circonus api url (e.g. inside).
	KeyAPIURL = "api.url"

	// KeyDebug enables debug messages.
	KeyDebug = "debug"
	// KeyDebugCGM enables debug messages for circonus-gometrics.
	KeyDebugCGM = "debug_cgm"

	// KeyDebugAPI enables debug messages for circonus API calls.
	KeyDebugAPI = "debug_api"

	// KeyDebugDumpMetrics enables dumping metrics to a file as they are submitted to circonus
	// it should contain a directory name where the user running circonus-agentd has write
	// permissions. metrics will be dumped for each _successful_ request.
	KeyDebugDumpMetrics = "debug_dump_metrics"

	// Process metrics will be included if EITHER cpu or mem utilization percentages are above these thresholds
	// KeyCPUThreshold high cpu utilization percentage to include process metrics.
	KeyCPUThreshold = "thresholds.high_cpu"
	// KeyMemThreshold high memory utilization percentage to include process metrics.
	KeyMemThreshold = "thresholds.high_memory"

	// KeyListen primary address and port to listen on.
	KeyListen = "listen"

	// KeyListenSocket identifies one or more unix socket files to create.
	KeyListenSocket = "listen_socket"

	// KeyLogLevel logging level (panic, fatal, error, warn, info, debug, disabled).
	KeyLogLevel = "log.level"

	// KeyLogPretty output formatted log lines (for running in foreground).
	KeyLogPretty = "log.pretty"

	// KeyPluginDir plugin directory.
	KeyPluginDir = "plugin_dir"
	// KeyPluginList is a list of explicit commands to run as plugins.
	KeyPluginList = "plugin_list"

	// KeyPluginTTLUnits plugin run ttl units.
	KeyPluginTTLUnits = "plugin_ttl_units"

	// KeyMultiAgent indicates whether multiple agents will be sending metrics to a single check (requires enterprise brokers).
	KeyMultiAgent = "multi_agent.enabled"

	// KeyMultiAgentInterval indicates how frequently metrics should be sent.
	KeyMultiAgentInterval = "multi_agent.interval"

	// KeyMultiAgentAccumulate determines if metrics are accumulated on the broker.
	KeyMultiAgentAccumulate = "multi_agent.accumulate"

	// KeyReverse indicates whether to use reverse connections.
	KeyReverse = "reverse.enabled"

	// KeyReverseBrokerCAFile custom broker ca file.
	KeyReverseBrokerCAFile = "reverse.broker_ca_file"

	// KeyReverseMaxConnRetry how many times to retry a persistently failing broker connection. default 10, -1 = indefinitely.
	KeyReverseMaxConnRetry = "reverse.max_conn_retry"

	// KeyShowConfig - show configuration and exit.
	KeyShowConfig = "show-config"

	// KeyShowVersion - show version information and exit.
	KeyShowVersion = "version"

	// KeySSLCertFile pem certificate file for SSL.
	KeySSLCertFile = "ssl.cert_file"

	// KeySSLKeyFile key for ssl.cert_file.
	KeySSLKeyFile = "ssl.key_file"

	// KeySSLListen ssl address and prot to listen on.
	KeySSLListen = "ssl.listen"

	// KeySSLVerify controls verification for ssl connections.
	KeySSLVerify = "ssl.verify"

	// KeyStatsdDisabled disables the default statsd listener.
	KeyStatsdDisabled = "statsd.disabled"

	// KeyStatsdGroupCID circonus check id for "group" metrics sent to statsd.
	KeyStatsdGroupCID = "statsd.group.check_id"

	// KeyStatsdGroupCounters operator for group counters (sum|average).
	KeyStatsdGroupCounters = "statsd.group.counters"

	// KeyStatsdGroupGauges operator for group gauges (sum|average).
	KeyStatsdGroupGauges = "statsd.group.gauges"

	// KeyStatsdGroupPrefix metrics prefixed with this string are considered "group" metrics.
	KeyStatsdGroupPrefix = "statsd.group.metric_prefix"

	// KeyStatsdGroupSets operator for group sets (sum|average).
	KeyStatsdGroupSets = "statsd.group.sets"

	// KeyStatsdHostCategory "plugin" name to put metrics sent to host.
	KeyStatsdHostCategory = "statsd.host.category"

	// KeyStatsdHostPrefix metrics prefixed with this string are considered "host" metrics.
	KeyStatsdHostPrefix = "statsd.host.metric_prefix"

	// KeyStatsdAddr address for statsd listener (default address will always be 'localhost').
	KeyStatsdAddr = "statsd.addr"

	// KeyStatsdPort port for statsd listener.
	KeyStatsdPort = "statsd.port"

	// KeyStatsdEnableTCP enables statsd tcp listener.
	KeyStatsdEnableTCP = "statsd.enable_tcp"

	// KeyStatsdMaxTCPConns set max statsd tcp connections.
	KeyStatsdMaxTCPConns = "statsd.max_tcp_connections"

	// KeyStatsdNPP sets number of packet processors to run concurrently.
	KeyStatsdNPP = "statsd.npp"

	// KeyStatsdPQS sets packet queue size.
	KeyStatsdPQS = "statsd.pqs"

	// KeyStatsdDebugMetricParsing controls verbose logging of metric parsing results.
	KeyStatsdDebugMetricParsing = "statsd.debug_metric_parsing"

	// KeyCollectors defines the builtin collectors to enable.
	KeyCollectors = "collectors"
	// KeyHostProc defines path builtins will use.
	KeyHostProc = "host_proc"
	// KeyHostSys defines path builtins will use, if needed.
	KeyHostSys = "host_sys"
	// KeyHostEtc defines path builtins will use, if needed.
	KeyHostEtc = "host_etc"
	// KeyHostVar defines path builtins will use, if needed.
	KeyHostVar = "host_var"
	// KeyHostRun defines path builtins will use, if needed.
	KeyHostRun = "host_run"

	// KeyDisableGzip disables gzip on http responses.
	KeyDisableGzip = "server.disable_gzip"

	// KeyCheckBundleID the check bundle id to use.
	KeyCheckBundleID = "check.bundle_id"

	// KeyCheckTarget the check bundle target to use to search for or create a check bundle
	// note: if not using reverse, this must be an IP address reachable by the broker.
	KeyCheckTarget = "check.target"

	// KeyCheckEnableNewMetrics toggles automatically enabling new metrics
	// KeyCheckEnableNewMetrics = "check.enable_new_metrics"
	// KeyCheckMetricStateDir defines the path where check metric state will be maintained when --check-enable-new-metrics is turned on
	// KeyCheckMetricStateDir = "check.metric_state_dir"
	// KeyCheckMetricRefreshTTL determines how often to refresh check bundle metrics from API when enable new metrics is turned on
	// KeyCheckMetricRefreshTTL = "check.metric_refresh_ttl"
	// KeyMetricFilters sets the filters used to automatically enable metrics on NEW checks.
	// The format [][]string{[]string{"allow|deny","rule_regex(pcre)","comment"},...}
	// If no metric filters are provided and enable new metrics is turned on. When creating a
	// new check a default set of filters will be used. (`[][]string{[]string{"deny","^$",""},[]string{"allow","^.+$",""}}`
	// thereby allowing all metrics.) See: "Metric Filters" section of https://login.circonus.com/resources/api/calls/check_bundle
	// for more information on filters. When filters are used, the agent will
	// NOT interact with the API to update the check to enable metrics. (the MetricStateDir
	// and MetricRefreshTTL are not applicable and will be ignored/deprecated going forward.)
	// The syntax for filters is embedded json (metric filters are positional, first match wins):
	// command line or environment variable
	//  `CA_CHECK_METRIC_FILTERS='[["deny","^$",""],["allow","^.+$",""]]'`
	//  `--check-metric-filters='[["deny","^$",""],["allow","^.+$",""]]'`
	// JSON configuration file:
	//  `"metric_filters": "[[\"deny\",\"^$\",\"\"],[\"allow\",\"^.+$\",\"\"]]"`
	// YAML configuration file:
	//  `metric_filters: '[["deny","^$",""],["allow","^.+$",""]]'`
	//  `metric_filters: '[["deny","^$",""],["deny","^.+$","tags","and(env:dev)",""],["allow","^.+$",""]]'`
	// TOML configuration file:
	//  `metric_filters = '''[["deny","$^",""],["allow","^.+$",""]]'''`
	KeyCheckMetricFilters = "check.metric_filters"
	// KeyCheckMetricFilterFile an external JSON file to use (see etc/example_metric_filters.json).
	KeyCheckMetricFilterFile = "check.metric_filter_file"
	// KeyCheckUpdateMetricFIlters force update of check with configured metric filters on start.
	KeyCheckUpdateMetricFilters = "check.update_metric_filters"

	// KeyCheckPeriod when broker requests metrics.
	KeyCheckPeriod = "check.period"

	// KeyCheckTimeout broker timeout when requesting metrics.
	KeyCheckTimeout = "check.timeout"

	// KeyCheckCreate toggles creating a new check bundle when a check bundle id is not supplied.
	KeyCheckCreate = "check.create"

	// KeyCheckUpdate forces updating all configurable check bundle attributes.
	KeyCheckUpdate = "check.update"

	// KeyCheckBroker a specific broker ID to use when creating a new check bundle.
	KeyCheckBroker = "check.broker"

	// KeyCheckTitle a specific title to use when creating a new check bundle.
	KeyCheckTitle = "check.title"

	// KeyCheckTags a specific set of tags to use when creating a new check bundle.
	KeyCheckTags = "check.tags"

	// KeyCheckMetricStreamtags specifies whether to use stream tags (if stream tags are enabled, check tags are added to all metrics by default).
	KeyCheckMetricStreamtags = "check.metric_streamtags"

	// KeyCheckDelete will allow the agent to delete its check if it had write access to etc/ when it created
	// its check so that it could save the check definition.
	KeyCheckDelete = "check.delete"

	// Cluster mode.
	KeyCluster = "cluster"
	// Cluster mode enabled.
	KeyClusterEnabled = "cluster.enabled"
	// Cluster mode enable builtins (host filesystems must be mounted in container and corresponding
	// `HOST_*`` environment variables must be set).
	KeyClusterEnableBuiltins = "cluster.enable_builtins"
	// Cluster mode represent statsd gauges as histogram samples, so that _one_ sample will be collected for each node.
	KeyClusterStatsdHistogramGauges = "cluster.statsd_histogram_gauges"

	cosiName = "cosi"
)

var (
	// MetricNameSeparator defines character used to delimit metric name parts.
	MetricNameSeparator = defaults.MetricNameSeparator // var, TBD whether it will become configurable
)

// Validate verifies the required portions of the configuration.
func Validate() error {

	if apiRequired() {
		err := validateAPIOptions()
		if err != nil {
			return fmt.Errorf("API config: %w", err)
		}
	}

	isReverse := viper.GetBool(KeyReverse)
	isMultiAgent := viper.GetBool(KeyMultiAgent)

	if isReverse && isMultiAgent {
		return fmt.Errorf("cannot enable reverse AND multi agent simultaneously") //nolint:goerr113
	}

	if isReverse {
		err := validateReverseOptions()
		if err != nil {
			return fmt.Errorf("reverse config: %w", err)
		}
	}

	if viper.GetString(KeyCheckBundleID) != "" && viper.GetBool(KeyCheckCreate) {
		return fmt.Errorf("use --check-create OR --check-id, they are mutually exclusive") //nolint:goerr113
	}

	return nil
}

// StatConfig adds the running config to the app stats.
func StatConfig() error {
	cfg, err := getConfig()
	if err != nil {
		return err
	}

	cfg.API.Key = "..."
	cfg.API.App = "..."

	expvar.Publish("config", expvar.Func(func() interface{} {
		return &cfg
	}))

	return nil
}

// getConfig dumps the current configuration and returns it.
func getConfig() (*Config, error) {
	var cfg *Config

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}

// ShowConfig prints the running configuration.
func ShowConfig(w io.Writer) error {
	var cfg *Config
	var err error
	var data []byte

	cfg, err = getConfig()
	if err != nil {
		return err
	}

	format := viper.GetString(KeyShowConfig)

	// log.Debug().Str("format", format).Msg("show-config")

	switch format {
	case "json":
		data, err = json.MarshalIndent(cfg, " ", "  ")
	case "yaml":
		data, err = yaml.Marshal(cfg)
	case "toml":
		data, err = toml.Marshal(*cfg)
	default:
		return fmt.Errorf("unknown config format '%s'", format) //nolint:goerr113
	}

	if err != nil {
		return fmt.Errorf("formatting config (%s): %w", format, err)
	}

	fmt.Fprintln(w, string(data))
	return nil
}
