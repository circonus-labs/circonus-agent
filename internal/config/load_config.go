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
	yaml "gopkg.in/yaml.v2"
)

type FileNotFoundErr struct {
	Name    string
	ExtList []string
}

func (e *FileNotFoundErr) Error() string {
	return fmt.Sprintf("no config found matching (%s%s)", e.Name, strings.Join(e.ExtList, "|"))
}

// LoadConfigFile will attempt to load json|toml|yaml configuration files.
// `base` is the full path and base name of the configuration file to load.
// `target` is an interface in to which the data will be loaded. Checks for
// '<base>.json', '<base>.toml', and '<base>.yaml'.
func LoadConfigFile(base string, target interface{}) error {

	if base == "" {
		return fmt.Errorf("invalid config file (empty)") //nolint:goerr113
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
			return fmt.Errorf("reading configuration file (%s): %w", cfg, err)
		}
		switch ext {
		case ".json":
			if err := json.Unmarshal(data, target); err != nil {
				return fmt.Errorf("parsing configuration file (%s): %w", cfg, err)
			}
			loaded = true
		case ".toml":
			if err := toml.Unmarshal(data, target); err != nil {
				return fmt.Errorf("parsing configuration file (%s): %w", cfg, err)
			}
			loaded = true
		case ".yaml":
			if err := yaml.Unmarshal(data, target); err != nil {
				return fmt.Errorf("parsing configuration file (%s): %w", cfg, err)
			}
			loaded = true
		}
	}

	if !loaded {
		return fmt.Errorf("no config found matching (%s%s): %w", base, strings.Join(extensions, "|"), os.ErrNotExist)
	}

	return nil
}
