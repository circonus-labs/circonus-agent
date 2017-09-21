// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"net"
	"os"
	"path/filepath"

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

func verifyFile(fileName string) (string, error) {
	if fileName == "" {
		return "", errors.New("Invalid file name (empty)")
	}

	var absFileName string
	var fi os.FileInfo
	var err error

	absFileName, err = filepath.Abs(fileName)
	if err != nil {
		return "", err
	}

	fileName = absFileName

	fi, err = os.Stat(fileName)
	if os.IsNotExist(err) {
		return "", err
	}

	if os.IsPermission(err) {
		return "", err
	}

	if !fi.Mode().IsRegular() {
		return "", errors.New("not a regular file")
	}

	// also try opening, to verify permissions
	// if last directory on path is not accessible to user, stat doesn't return EPERM
	f, err := os.Open(fileName)
	if err != nil {
		return "", err
	}
	f.Close()

	return fileName, nil
}
