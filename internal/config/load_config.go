// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	toml "github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

// LoadConfigFile will attempt to load json|toml|yaml configuration files.
// `base` is the full path and base name of the configuration file to load.
// `target` is an interface in to which the data will be loaded. Checks for
// '<base>.json', '<base>.toml', and '<base>.yaml'.
func LoadConfigFile(base string, target interface{}) error {

	if base == "" {
		return errors.Errorf("invalid config file (empty)")
	}

	extensions := []string{".json", ".toml", ".yaml"}
	loaded := false

	for _, ext := range extensions {
		cfg := base + ext
		if _, err := os.Stat(cfg); os.IsNotExist(err) {
			continue
		}
		data, err := ioutil.ReadFile(cfg)
		if err != nil {
			return errors.Wrapf(err, "reading configuration file (%s)", cfg)
		}
		parseErrMsg := fmt.Sprintf("parsing configuration file (%s)", cfg)
		switch ext {
		case ".json":
			if err := json.Unmarshal(data, target); err != nil {
				return errors.Wrap(err, parseErrMsg)
			}
			loaded = true
		case ".toml":
			if err := toml.Unmarshal(data, target); err != nil {
				return errors.Wrap(err, parseErrMsg)
			}
			loaded = true
		case ".yaml":
			if err := yaml.Unmarshal(data, target); err != nil {
				return errors.Wrap(err, parseErrMsg)
			}
			loaded = true
		}
	}

	if !loaded {
		return errors.Errorf("no config found matching (%s%s)", base, strings.Join(extensions, "|"))
	}

	return nil
}
