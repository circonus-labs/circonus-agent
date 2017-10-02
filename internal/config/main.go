// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"net"

	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
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
