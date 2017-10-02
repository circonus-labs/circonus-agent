// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"net"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func validateSSLOptions() error {
	sslSpec := viper.GetString(KeySSLListen)

	ip, port, err := parseListen(sslSpec, "")
	if err != nil {
		return err
	}

	viper.Set(KeySSLListen, net.JoinHostPort(ip, port))

	sslCert := viper.GetString(KeySSLCertFile)
	sslKey := viper.GetString(KeySSLKeyFile)

	cert, err := verifyFile(sslCert)
	if err != nil {
		return errors.Wrapf(err, "SSL cert")
	}

	key, err := verifyFile(sslKey)
	if err != nil {
		return errors.Wrapf(err, "SSL key")
	}

	viper.Set(KeySSLCertFile, cert)
	viper.Set(KeySSLKeyFile, key)

	return nil
}
