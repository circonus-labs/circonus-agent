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
func (p *Plugins) Scan() error {
	p.Lock()
	defer p.Unlock()

	// initialRun fires each plugin one time. Unlike 'Run' it does
	// not wait for plugins to finish this will provides:
	//
	// 1. an initial seeding of results
	// 2. starts any long running plugins without blocking
	//
	initialRun := func() error {
		for id, plug := range p.active {
			p.logger.Debug().
				Str("plugin", id).
				Msg("Initializing")
			go plug.exec()
		}
		return nil
	}

	if err := p.Stop(); err != nil {
		return errors.Wrap(err, "stopping plugin(s)")
	}

	if err := p.scanPluginDirectory(); err != nil {
		return errors.Wrap(err, "plugin directory scan")
	}

	if err := initialRun(); err != nil {
		return errors.Wrap(err, "initializing plugin(s)")
	}

	return nil
}

// scanPluginDirectory finds and loads plugins
func (p *Plugins) scanPluginDirectory() error {
	p.generation++

	pluginDir := viper.GetString(config.KeyPluginDir)

	if pluginDir == "" {
		return errors.New("invalid plugin directory (none)")
	}

	p.logger.Info().
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

		p.logger.Debug().
			Str("path", filepath.Join(pluginDir, fileName)).
			Msg("checking plugin directory entry")

		if fi.IsDir() {
			p.logger.Debug().
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
			p.logger.Debug().
				Str("file", fileName).
				Msg("invalid file name format, ignoring")
			continue
		}

		if fileExt == ".conf" || fileExt == ".json" {
			p.logger.Debug().
				Str("file", fileName).
				Msg("config file, ignoring")
			continue
		}

		if _, reserved := p.reservedNames[fileBase]; reserved {
			p.logger.Warn().
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
				p.logger.Warn().
					Err(err).
					Str("file", fi.Name()).
					Msg("Error resolving symlink, ignoring")
				continue
			}
			cmdName = resolvedSymlink
		default:
			p.logger.Debug().
				Str("file", fileName).
				Msg("not a regular file or symlink, ignoring")
			continue // just ignore it
		}

		if perm := fi.Mode().Perm() & 0111; perm != 73 {
			p.logger.Warn().
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
				p.logger.Warn().
					Err(err).
					Str("config", cfgFile).
					Str("plugin", fileBase).Msg("plugin config")
			}
		} else {
			if len(data) > 0 {
				err := json.Unmarshal(data, &cfg)
				if err != nil {
					p.logger.Warn().
						Err(err).
						Str("config", cfgFile).
						Str("plugin", fileBase).
						Str("data", string(data)).
						Msg("parsing config")
				}

				p.logger.Debug().
					Str("config", fmt.Sprintf("%+v", cfg)).
					Msg("loaded plugin config")
			}
		}

		if cfg == nil {
			plug, ok := p.active[fileBase]
			if !ok {
				p.active[fileBase] = &plugin{
					ctx:    p.ctx,
					ID:     fileBase,
					Name:   fileBase,
					logger: p.logger.With().Str("plugin", fileBase).Logger(),
					RunDir: p.pluginDir,
				}
				plug = p.active[fileBase]
			}

			plug.Generation = p.generation
			plug.Command = cmdName
			p.logger.Info().
				Str("id", fileBase).
				Str("cmd", cmdName).
				Uint64("generation", p.generation).
				Msg("Activating plugin")

		} else {
			for inst, args := range cfg {
				pluginName := fmt.Sprintf("%s`%s", fileBase, inst)
				plug, ok := p.active[pluginName]
				if !ok {
					p.active[pluginName] = &plugin{
						ctx:          p.ctx,
						ID:           fileBase,
						InstanceID:   inst,
						InstanceArgs: args,
						Name:         pluginName,
						logger:       p.logger.With().Str("plugin", pluginName).Logger(),
						RunDir:       p.pluginDir,
					}
					plug = p.active[pluginName]
				}

				plug.Generation = p.generation
				plug.Command = cmdName
				p.logger.Info().
					Str("id", pluginName).
					Str("cmd", cmdName).
					Uint64("generation", p.generation).
					Msg("Activating plugin")

			}
		}
	}

	// purge inactive plugins (plugins removed from plugin directory)
	for id, plug := range p.active {
		if plug.Generation != p.generation {
			p.logger.Debug().
				Str("plugin", id).
				Msg("purging inactive plugin")
			delete(p.active, id)
		}
	}

	return nil
}
