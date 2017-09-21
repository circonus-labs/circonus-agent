// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package plugins

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// Scan the plugin directory for new/updated plugins
func Scan() error {
	pluginList.Lock()
	defer pluginList.Unlock()

	// stop will kill any long running plugins in preparation
	// for rescanning the plugin directory. IF watching for
	// changes is added (TBD - restarting is preferred, cleaner).
	stop := func() error {
		for id, plug := range pluginList.active {
			if plug.cmd != nil && !plug.cmd.ProcessState.Exited() {
				logger.Debug().
					Str("plugin", id).
					Msg("Stopping long running plugin")
				err := plug.cmd.Process.Kill()
				if err != nil {
					logger.Error().
						Err(err).
						Str("plugin", id).
						Msg("Stopping plugin")
				}
			}
		}
		return nil
	}

	// initialRun fires each plugin one time. Unlike 'Run' it does
	// not wait for plugins to finish this will provides:
	//
	// 1. an initial seeding of results
	// 2. starts any long running plugins without blocking
	//
	initialRun := func() error {
		for id, plug := range pluginList.active {
			logger.Debug().
				Str("plugin", id).
				Msg("Initializing")
			go plug.exec()
		}
		return nil
	}

	if err := stop(); err != nil {
		return errors.Wrap(err, "stopping plugin(s)")
	}

	if err := pluginList.scanPluginDirectory(); err != nil {
		return errors.Wrap(err, "plugin directory scan")
	}

	if err := initialRun(); err != nil {
		return errors.Wrap(err, "initializing plugin(s)")
	}

	return nil
}

// scanPluginDirectory finds and loads plugins
func (pl *PluginList) scanPluginDirectory() error {
	pl.generation++

	pluginDir := viper.GetString(config.KeyPluginDir)

	if pluginDir == "" {
		return errors.New("invalid plugin directory (none)")
	}

	logger.Info().
		Str("dir", pluginDir).
		Msg("Scanning plugin directory")

	f, err := os.Open(pluginDir)
	if err != nil {
		return errors.Wrap(err, "open plugin directory")
	}

	defer f.Close()

	files, err := f.Readdir(-1)
	if err != nil {
		return errors.Wrap(err, "reading plugin directory")
	}

	for _, fi := range files {
		fileName := fi.Name()

		logger.Debug().
			Str("path", filepath.Join(pluginDir, fileName)).
			Msg("checking plugin directory entry")

		if fi.IsDir() {
			logger.Debug().
				Str("file", fileName).
				Msg("directory, ignoring")
			continue
		}

		fileBase := fileName
		fileExt := filepath.Ext(fileName)

		if fileExt != "" {
			fileBase = strings.Replace(fileName, fileExt, "", -1)
		}

		if fileBase == "" || fileExt == "" {
			logger.Debug().
				Str("file", fileName).
				Msg("invalid file name format, ignoring")
			continue
		}

		if fileExt == ".conf" || fileExt == ".json" {
			logger.Debug().
				Str("file", fileName).
				Msg("config file, ignoring")
			continue
		}

		if _, reserved := reservedNames[fileBase]; reserved {
			logger.Warn().
				Str("file", fileName).
				Msg("reserved plugin name, ignoring")
			continue
		}

		var cmdName string

		switch mode := fi.Mode(); {
		case mode.IsRegular():
			cmdName = filepath.Join(pluginDir, fi.Name())
		case mode&os.ModeSymlink != 0:
			resolvedSymlink, err := filepath.EvalSymlinks(filepath.Join(pluginDir, fi.Name()))
			if err != nil {
				logger.Warn().
					Err(err).
					Str("file", fi.Name()).
					Msg("Error resolving symlink, ignoring")
				continue
			}
			cmdName = resolvedSymlink
		default:
			logger.Debug().
				Str("file", fileName).
				Msg("not a regular file or symlink, ignoring")
			continue // just ignore it
		}

		if perm := fi.Mode().Perm() & 0111; perm != 73 {
			logger.Warn().
				Str("file", cmdName).
				Str("perms", fmt.Sprintf("%q", fi.Mode().Perm())).
				Msg("executable bit not set, ignoring")
			continue
		}

		var cfg map[string][]string

		// check for config file
		cfgFile := filepath.Join(pluginDir, fmt.Sprintf("%s.json", fileBase))
		if data, err := ioutil.ReadFile(cfgFile); err != nil {
			if !os.IsNotExist(err) {
				logger.Warn().
					Err(err).
					Str("config", cfgFile).
					Str("plugin", fileBase).Msg("plugin config")
			}
		} else {
			if len(data) > 0 {
				err := json.Unmarshal(data, &cfg)
				if err != nil {
					logger.Warn().
						Err(err).
						Str("config", cfgFile).
						Str("plugin", fileBase).
						Str("data", string(data)).
						Msg("parsing config")
				}

				logger.Debug().
					Str("config", fmt.Sprintf("%+v", cfg)).
					Msg("loaded plugin config")
			}
		}

		if cfg == nil {
			plug, ok := pl.active[fileBase]
			if !ok {
				pl.active[fileBase] = &Plugin{
					ID:     fileBase,
					Name:   fileBase,
					logger: logger.With().Str("plugin", fileBase).Logger(),
				}
				plug = pl.active[fileBase]
			}

			plug.Generation = pl.generation
			plug.Command = cmdName
			logger.Info().
				Str("id", fileBase).
				Str("cmd", cmdName).
				Uint64("generation", pl.generation).
				Msg("Activating plugin")

		} else {
			for inst, args := range cfg {
				pluginName := fmt.Sprintf("%s`%s", fileBase, inst)
				plug, ok := pl.active[pluginName]
				if !ok {
					pl.active[pluginName] = &Plugin{
						ID:           fileBase,
						InstanceID:   inst,
						InstanceArgs: args,
						Name:         pluginName,
						logger:       logger.With().Str("plugin", pluginName).Logger(),
					}
					plug = pl.active[pluginName]
				}

				plug.Generation = pl.generation
				plug.Command = cmdName
				logger.Info().
					Str("id", pluginName).
					Str("cmd", cmdName).
					Uint64("generation", pl.generation).
					Msg("Activating plugin")

			}
		}
	}

	// purge inactive plugins (plugins removed from plugin directory)
	for id, p := range pl.active {
		if p.Generation != pl.generation {
			logger.Debug().
				Str("plugin", id).
				Msg("purging inactive plugin")
			delete(pl.active, id)
		}
	}

	return nil
}
