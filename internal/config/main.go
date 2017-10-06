// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"encoding/json"
	"expvar"
	"fmt"
	"io"
	"net"

	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-agent/internal/release"
	toml "github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
)

// Validate verifies the required portions of the configuration
func Validate() error {

	if err := validatePluginDirectory(); err != nil {
		return errors.Wrap(err, "plugin directory config")
	}

	log.Debug().
		Str("path", viper.GetString(KeyPluginDir)).
		Msg("plugin directory")

	if viper.GetString(KeySSLListen) != "" {
		if err := validateSSLOptions(); err != nil {
			return errors.Wrap(err, "ssl server config")
		}

		log.Debug().
			Str("listen", viper.GetString(KeySSLListen)).
			Str("cert", viper.GetString(KeySSLCertFile)).
			Str("key", viper.GetString(KeySSLKeyFile)).
			Bool("verify", viper.GetBool(KeySSLVerify)).
			Msg("ssl options")
	}

	if apiRequired() {
		err := validateAPIOptions()
		if err != nil {
			return errors.Wrap(err, "API config")
		}
	}

	if viper.GetBool(KeyReverse) {
		err := validateReverseOptions()
		if err != nil {
			return errors.Wrap(err, "reverse config")
		}
	}

	if !viper.GetBool(KeyStatsdDisabled) {
		if err := validateStatsdOptions(); err != nil {
			return errors.Wrap(err, "StatsD config")
		}
	}

	listenSpec := viper.GetString(KeyListen)
	if listenSpec == "" && viper.GetString(KeySSLListen) != "" {
		return nil // only ssl
	}

	ip, port, err := parseListen(listenSpec, defaults.Listen)
	if err != nil {
		return errors.Wrap(err, "server config")
	}
	viper.Set(KeyListen, net.JoinHostPort(ip, port))
	log.Debug().
		Str("listen", viper.GetString(KeyListen)).
		Msg("server config")

	return nil
}

// StatConfig adds the running config to the app stats
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

// getConfig dumps the current configuration and returns it
func getConfig() (*Config, error) {
	var cfg *Config

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, errors.Wrap(err, "parsing config")
	}

	return cfg, nil
}

// ShowConfig prints the running configuration
func ShowConfig(w io.Writer) error {
	var cfg *Config
	var err error
	var data []byte

	cfg, err = getConfig()
	if err != nil {
		return err
	}

	format := viper.GetString(KeyShowConfig)

	log.Debug().Str("format", format).Msg("show-config")

	switch format {
	case "json":
		data, err = json.MarshalIndent(cfg, " ", "  ")
		if err != nil {
			return errors.Wrap(err, "formatting config (json)")
		}
	case "yaml":
		data, err = yaml.Marshal(cfg)
		if err != nil {
			return errors.Wrap(err, "formatting config (yaml)")
		}
	case "toml":
		data, err = toml.Marshal(*cfg)
		if err != nil {
			return errors.Wrap(err, "formatting config (toml)")
		}
	default:
		return errors.Errorf("unknown config format '%s'", format)
	}

	fmt.Fprintf(w, "%s v%s running config:\n%s\n", release.NAME, release.VERSION, data)
	return nil
}
