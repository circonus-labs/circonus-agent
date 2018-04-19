// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package cmd

import (
	"fmt"
	stdlog "log"
	"os"
	"runtime"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/agent"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   release.NAME,
	Short: "Circonus Host Agent",
	Long: `The Circonus host agent daemon provides a simple mechanism
to expose systems and application metrics to Circonus.
It inventories all executable programs in its plugin directory
and executes them upon external request, returning results
in JSON format.`,
	PersistentPreRunE: initLogging,
	Run: func(cmd *cobra.Command, args []string) {
		//
		// show version and exit
		//
		if viper.GetBool(config.KeyShowVersion) {
			fmt.Printf("%s v%s - commit: %s, date: %s, tag: %s\n", release.NAME, release.VERSION, release.COMMIT, release.DATE, release.TAG)
			return
		}

		//
		// show configuration and exit
		//
		if viper.GetString(config.KeyShowConfig) != "" {
			if err := config.ShowConfig(os.Stdout); err != nil {
				log.Fatal().Err(err).Msg("show-config")
			}
			return
		}

		log.Info().
			Int("pid", os.Getpid()).
			Str("name", release.NAME).
			Str("ver", release.VERSION).Msg("Starting")

		a, err := agent.New()
		if err != nil {
			log.Fatal().Err(err).Msg("initializing")
		}

		config.StatConfig()

		if err := a.Start(); err != nil {
			log.Fatal().Err(err).Msg("starting agent")
		}
	},
}

func init() {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zlog := zerolog.New(zerolog.SyncWriter(os.Stderr)).With().Timestamp().Logger()
	log.Logger = zlog

	stdlog.SetFlags(0)
	stdlog.SetOutput(zlog)

	cobra.OnInitialize(initConfig)

	desc := func(desc, env string) string {
		return fmt.Sprintf("[ENV: %s] %s", env, desc)
	}

	//
	// Basic
	//
	{
		var (
			longOpt     = "config"
			shortOpt    = "c"
			description = "config file (default is " + defaults.EtcPath + "/" + release.NAME + ".(json|toml|yaml)"
		)
		RootCmd.PersistentFlags().StringVarP(&cfgFile, longOpt, shortOpt, "", description)
	}

	{
		var (
			key         = config.KeyListen
			longOpt     = "listen"
			shortOpt    = "l"
			envVar      = release.ENVPREFIX + "_LISTEN"
			description = "Listen spec e.g. :2609, [::1], [::1]:2609, 127.0.0.1, 127.0.0.1:2609, foo.bar.baz, foo.bar.baz:2609 " + `(default "` + defaults.Listen + `")`
		)

		RootCmd.Flags().StringSliceP(longOpt, shortOpt, []string{}, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
	}

	{
		var (
			key         = config.KeyCollectors
			longOpt     = "collectors"
			envVar      = release.ENVPREFIX + "_COLLECTORS"
			description = "List of builtin collectors to enable"
		)

		RootCmd.Flags().StringSlice(longOpt, defaults.Collectors, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.Collectors)
	}

	{
		const (
			key         = config.KeyListenSocket
			longOpt     = "listen-socket"
			shortOpt    = "L"
			envVar      = release.ENVPREFIX + "_LISTEN_SOCKET"
			description = "Unix socket to create"
		)

		RootCmd.Flags().StringSliceP(longOpt, shortOpt, []string{}, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
	}

	{
		const (
			key         = config.KeyPluginDir
			longOpt     = "plugin-dir"
			shortOpt    = "p"
			envVar      = release.ENVPREFIX + "_PLUGIN_DIR"
			description = "Plugin directory"
		)

		RootCmd.Flags().StringP(longOpt, shortOpt, defaults.PluginPath, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.PluginPath)
	}

	{
		const (
			key         = config.KeyPluginTTLUnits
			longOpt     = "plugin-ttl-units"
			envVar      = release.ENVPREFIX + "_PLUGIN_TTL_UNITS"
			description = "Default plugin TTL units"
		)

		RootCmd.Flags().String(longOpt, defaults.PluginTTLUnits, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.PluginTTLUnits)
	}

	//
	// Reverse mode
	//
	{
		const (
			key         = config.KeyReverse
			longOpt     = "reverse"
			shortOpt    = "r"
			envVar      = release.ENVPREFIX + "_REVERSE"
			description = "Enable reverse connection"
		)

		RootCmd.Flags().BoolP(longOpt, shortOpt, defaults.Reverse, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.Reverse)
	}

	{
		const (
			key          = config.KeyReverseBrokerCAFile
			longOpt      = "reverse-broker-ca-file"
			defaultValue = ""
			envVar       = release.ENVPREFIX + "_REVERSE_BROKER_CA_FILE"
			description  = "Broker CA certificate file"
		)

		RootCmd.Flags().String(longOpt, defaultValue, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
	}

	//
	// Check
	//
	{
		const (
			key          = config.KeyCheckBundleID
			longOpt      = "check-id"
			defaultValue = ""
			shortOpt     = "I"
			envVar       = release.ENVPREFIX + "_CHECK_ID"
			description  = "Check Bundle ID or 'cosi' for cosi system check (for reverse and auto enable new metrics)"
		)

		RootCmd.Flags().StringP(longOpt, shortOpt, defaultValue, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
	}

	{
		const (
			key         = config.KeyCheckCreate
			longOpt     = "check-create"
			shortOpt    = "C"
			envVar      = release.ENVPREFIX + "_CHECK_CREATE"
			description = "Create check bundle (for reverse and auto enable new metrics)"
		)

		RootCmd.Flags().BoolP(longOpt, shortOpt, defaults.CheckCreate, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.CheckCreate)
	}

	{
		const (
			key         = config.KeyCheckTarget
			longOpt     = "check-target"
			shortOpt    = "T"
			envVar      = release.ENVPREFIX + "_CHECK_TARGET"
			description = "Check target host (for creating a new check)"
		)

		RootCmd.Flags().StringP(longOpt, shortOpt, defaults.CheckTarget, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.CheckTarget)

	}

	{
		const (
			key         = config.KeyCheckTitle
			longOpt     = "check-title"
			envVar      = release.ENVPREFIX + "_CHECK_TITLE"
			description = "Title [display name] to use, if creating a check bundle (default \"<check-target> /agent\")"
		)

		RootCmd.Flags().String(longOpt, "", desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
	}

	{
		const (
			key         = config.KeyCheckBroker
			longOpt     = "check-broker"
			envVar      = release.ENVPREFIX + "_CHECK_BROKER"
			description = "ID of Broker to use or 'select' for random selection of valid broker, if creating a check bundle"
		)

		RootCmd.Flags().String(longOpt, defaults.CheckBroker, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.CheckBroker)
	}

	{
		const (
			key         = config.KeyCheckTags
			longOpt     = "check-tags"
			envVar      = release.ENVPREFIX + "_CHECK_TAGS"
			description = "Tags [comma separated list] to use, if creating a check bundle"
		)

		RootCmd.Flags().String(longOpt, defaults.CheckTags, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.CheckTags)
	}

	{
		const (
			key         = config.KeyCheckEnableNewMetrics
			longOpt     = "check-enable-new-metrics"
			shortOpt    = "E"
			envVar      = release.ENVPREFIX + "_CHECK_ENABLE_NEW_METRICS"
			description = "Automatically enable all new metrics"
		)

		RootCmd.Flags().BoolP(longOpt, shortOpt, defaults.CheckEnableNewMetrics, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.CheckEnableNewMetrics)
	}

	{
		const (
			key         = config.KeyCheckMetricRefreshTTL
			longOpt     = "check-metric-refresh-ttl"
			envVar      = release.ENVPREFIX + "_CHECK_METRIC_REFRESH_TTL"
			description = "Refresh check metrics TTL"
		)

		RootCmd.Flags().String(longOpt, defaults.CheckMetricRefreshTTL, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.CheckMetricRefreshTTL)
	}

	//
	// API
	//
	{
		const (
			key          = config.KeyAPITokenKey
			longOpt      = "api-key"
			defaultValue = ""
			envVar       = release.ENVPREFIX + "_API_KEY"
			description  = "Circonus API Token key"
		)
		RootCmd.Flags().String(longOpt, defaultValue, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
	}

	{
		const (
			key         = config.KeyAPITokenApp
			longOpt     = "api-app"
			envVar      = release.ENVPREFIX + "_API_APP"
			description = "Circonus API Token app"
		)

		RootCmd.Flags().String(longOpt, defaults.APIApp, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.APIApp)
	}

	{
		const (
			key         = config.KeyAPIURL
			longOpt     = "api-url"
			envVar      = release.ENVPREFIX + "_API_URL"
			description = "Circonus API URL"
		)

		RootCmd.Flags().String(longOpt, defaults.APIURL, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.APIURL)
	}

	{
		const (
			key          = config.KeyAPICAFile
			longOpt      = "api-ca-file"
			defaultValue = ""
			envVar       = release.ENVPREFIX + "_API_CA_FILE"
			description  = "Circonus API CA certificate file"
		)

		RootCmd.Flags().String(longOpt, defaultValue, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
	}

	//
	// SSL
	//
	{
		const (
			key          = config.KeySSLListen
			longOpt      = "ssl-listen"
			defaultValue = ""
			envVar       = release.ENVPREFIX + "_SSL_LISTEN"
			description  = "SSL listen address and port [IP]:[PORT] - setting enables SSL"
		)

		RootCmd.Flags().String(longOpt, defaultValue, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
	}

	{
		const (
			key         = config.KeySSLCertFile
			longOpt     = "ssl-cert-file"
			envVar      = release.ENVPREFIX + "_SSL_CERT_FILE"
			description = "SSL Certificate file (PEM cert and CAs concatenated together)"
		)

		RootCmd.Flags().String(longOpt, defaults.SSLCertFile, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.SSLCertFile)
	}

	{
		const (
			key         = config.KeySSLKeyFile
			longOpt     = "ssl-key-file"
			envVar      = release.ENVPREFIX + "_SSL_KEY_FILE"
			description = "SSL Key file"
		)

		RootCmd.Flags().String(longOpt, defaults.SSLKeyFile, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.SSLKeyFile)
	}

	{
		const (
			key         = config.KeySSLVerify
			longOpt     = "ssl-verify"
			envVar      = release.ENVPREFIX + "_SSL_VERIFY"
			description = "Enable SSL verification"
		)

		RootCmd.Flags().Bool(longOpt, defaults.SSLVerify, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.SSLVerify)
	}

	//
	// StatsD
	//
	{
		const (
			key         = config.KeyStatsdDisabled
			longOpt     = "no-statsd"
			envVar      = release.ENVPREFIX + "_NO_STATSD"
			description = "Disable StatsD listener"
		)

		RootCmd.Flags().Bool(longOpt, defaults.NoStatsd, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.NoStatsd)
	}

	{
		const (
			key         = config.KeyStatsdPort
			longOpt     = "statsd-port"
			envVar      = release.ENVPREFIX + "_STATSD_PORT"
			description = "StatsD port"
		)

		RootCmd.Flags().String(longOpt, defaults.StatsdPort, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.StatsdPort)
	}

	{
		const (
			key         = config.KeyStatsdHostPrefix
			longOpt     = "statsd-host-prefix"
			envVar      = release.ENVPREFIX + "_STATSD_HOST_PREFIX"
			description = "StatsD host metric prefix"
		)

		RootCmd.Flags().String(longOpt, defaults.StatsdHostPrefix, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.StatsdHostPrefix)
	}

	{
		const (
			key         = config.KeyStatsdHostCategory
			longOpt     = "statsd-host-cateogry"
			envVar      = release.ENVPREFIX + "_STATSD_HOST_CATEGORY"
			description = "StatsD host metric category"
		)

		RootCmd.Flags().String(longOpt, defaults.StatsdHostCategory, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.StatsdHostCategory)
	}

	{
		const (
			key          = config.KeyStatsdGroupCID
			longOpt      = "statsd-group-cid"
			defaultValue = ""
			envVar       = release.ENVPREFIX + "_STATSD_GROUP_CID"
			description  = "StatsD group check bundle ID"
		)

		RootCmd.Flags().String(longOpt, defaultValue, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
	}

	{
		const (
			key         = config.KeyStatsdGroupPrefix
			longOpt     = "statsd-group-prefix"
			envVar      = release.ENVPREFIX + "_STATSD_GROUP_PREFIX"
			description = "StatsD group metric prefix"
		)

		RootCmd.Flags().String(longOpt, defaults.StatsdGroupPrefix, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.StatsdGroupPrefix)
	}

	{
		const (
			key         = config.KeyStatsdGroupCounters
			longOpt     = "statsd-group-counters"
			envVar      = release.ENVPREFIX + "_STATSD_GROUP_COUNTERS"
			description = "StatsD group metric counter handling (average|sum)"
		)

		RootCmd.Flags().String(longOpt, defaults.StatsdGroupCounters, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.StatsdGroupCounters)
	}

	{
		const (
			key         = config.KeyStatsdGroupGauges
			longOpt     = "statsd-group-gauges"
			envVar      = release.ENVPREFIX + "_STATSD_GROUP_GAUGES"
			description = "StatsD group gauge operator"
		)

		RootCmd.Flags().String(longOpt, defaults.StatsdGroupGauges, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.StatsdGroupGauges)
	}

	{
		const (
			key         = config.KeyStatsdGroupSets
			longOpt     = "statsd-group-sets"
			envVar      = release.ENVPREFIX + "_STATSD_GROPUP_SETS"
			description = "StatsD group set operator"
		)

		RootCmd.Flags().String(longOpt, defaults.StatsdGroupSets, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.StatsdGroupSets)
	}

	// Miscellenous

	{
		const (
			key         = config.KeyDisableGzip
			longOpt     = "no-gzip"
			envVar      = release.ENVPREFIX + "_NO_GZIP"
			description = "Disable gzip HTTP responses"
		)

		RootCmd.Flags().Bool(longOpt, defaults.DisableGzip, description)
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.DisableGzip)
	}

	{
		const (
			key         = config.KeyDebug
			longOpt     = "debug"
			shortOpt    = "d"
			envVar      = release.ENVPREFIX + "_DEBUG"
			description = "Enable debug messages"
		)

		RootCmd.Flags().BoolP(longOpt, shortOpt, defaults.Debug, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.Debug)
	}

	{
		const (
			key          = config.KeyDebugCGM
			longOpt      = "debug-cgm"
			defaultValue = false
			envVar       = release.ENVPREFIX + "_DEBUG_CGM"
			description  = "Enable CGM & API debug messages"
		)

		RootCmd.Flags().Bool(longOpt, defaultValue, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key         = config.KeyDebugDumpMetrics
			longOpt     = "debug-dump-metrics"
			envVar      = release.ENVPREFIX + "_DEBUG_DUMP_METRICS"
			description = "Directory to dump sent metrics"
		)

		RootCmd.Flags().String(longOpt, "", desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
	}

	{
		const (
			key         = config.KeyLogLevel
			longOpt     = "log-level"
			envVar      = release.ENVPREFIX + "_LOG_LEVEL"
			description = "Log level [(panic|fatal|error|warn|info|debug|disabled)]"
		)

		RootCmd.Flags().String(longOpt, defaults.LogLevel, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.LogLevel)
	}

	{
		const (
			key         = config.KeyLogPretty
			longOpt     = "log-pretty"
			envVar      = release.ENVPREFIX + "_LOG_PRETTY"
			description = "Output formatted/colored log lines [ignored on windows]"
		)

		RootCmd.Flags().Bool(longOpt, defaults.LogPretty, desc(description, envVar))
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
		viper.BindEnv(key, envVar)
		viper.SetDefault(key, defaults.LogPretty)
	}

	// RootCmd.Flags().Bool("watch", defaults.Watch, "Watch plugins, reload on change")
	// viper.SetDefault("watch", defaults.Watch)
	// viper.BindPFlag("watch", RootCmd.Flags().Lookup("watch"))

	{
		const (
			key          = config.KeyShowVersion
			longOpt      = "version"
			shortOpt     = "V"
			defaultValue = false
			description  = "Show version and exit"
		)
		RootCmd.Flags().BoolP(longOpt, shortOpt, defaultValue, description)
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
	}

	{
		const (
			key         = config.KeyShowConfig
			longOpt     = "show-config"
			description = "Show config (json|toml|yaml) and exit"
		)

		RootCmd.Flags().String(longOpt, "", description)
		viper.BindPFlag(key, RootCmd.Flags().Lookup(longOpt))
	}
}

// initLogging initializes zerolog
func initLogging(cmd *cobra.Command, args []string) error {
	//
	// Enable formatted output
	//
	if viper.GetBool(config.KeyLogPretty) {
		if runtime.GOOS != "windows" {
			log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
		} else {
			log.Warn().Msg("log-pretty not applicable on this platform")
		}
	}

	//
	// Enable debug logging, if requested
	// otherwise, default to info level and set custom level, if specified
	//
	if viper.GetBool(config.KeyDebug) {
		viper.Set(config.KeyLogLevel, "debug")
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log.Debug().Msg("--debug flag, forcing debug log level")
	} else {
		if viper.IsSet(config.KeyLogLevel) {
			level := viper.GetString(config.KeyLogLevel)

			switch level {
			case "panic":
				zerolog.SetGlobalLevel(zerolog.PanicLevel)
			case "fatal":
				zerolog.SetGlobalLevel(zerolog.FatalLevel)
			case "error":
				zerolog.SetGlobalLevel(zerolog.ErrorLevel)
			case "warn":
				zerolog.SetGlobalLevel(zerolog.WarnLevel)
			case "info":
				zerolog.SetGlobalLevel(zerolog.InfoLevel)
			case "debug":
				zerolog.SetGlobalLevel(zerolog.DebugLevel)
			case "disabled":
				zerolog.SetGlobalLevel(zerolog.Disabled)
			default:
				return errors.Errorf("Unknown log level (%s)", level)
			}

			log.Debug().Str("log-level", level).Msg("Logging level")
		}
	}

	return nil
}

// initConfig reads in config file and/or ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(defaults.EtcPath)
		viper.AddConfigPath(".")
		viper.SetConfigName(release.NAME)
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		f := viper.ConfigFileUsed()
		if f != "" {
			log.Fatal().Err(err).Str("config_file", f).Msg("Unable to load config file")
		}
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.Fatal().
			Err(err).
			Msg("Unable to start")
	}
}
