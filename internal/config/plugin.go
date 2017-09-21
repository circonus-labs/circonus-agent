// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func validatePluginDirectory() error {
	errMsg := "Invalid plugin directory"

	pluginDir := viper.GetString(KeyPluginDir)

	if pluginDir == "" {
		return errors.Errorf(errMsg+" (%s)", pluginDir)
	}

	absDir, err := filepath.Abs(pluginDir)
	if err != nil {
		return errors.Wrap(err, errMsg)
	}

	pluginDir = absDir

	fi, err := os.Stat(pluginDir)
	if err != nil {
		return errors.Wrap(err, errMsg)
	}

	if !fi.Mode().IsDir() {
		return errors.Errorf(errMsg+" (%s) not a directory", pluginDir)
	}

	// also try opening, to verify permissions
	// if last dir on path is not accessible to user, stat doesn't return EPERM
	f, err := os.Open(pluginDir)
	if err != nil {
		return errors.Wrap(err, errMsg)
	}
	f.Close()

	viper.Set(KeyPluginDir, pluginDir)

	return nil
}
