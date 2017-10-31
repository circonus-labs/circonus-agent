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
	"regexp"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/maier/go-appstats"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// Scan the plugin directory for new/updated plugins
func (p *Plugins) Scan(b *builtins.Builtins) error {
	p.Lock()
	defer p.Unlock()

	if p.pluginDir == "" {
		return nil
	}

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

	if err := p.scanPluginDirectory(b); err != nil {
		return errors.Wrap(err, "plugin directory scan")
	}

	if err := initialRun(); err != nil {
		return errors.Wrap(err, "initializing plugin(s)")
	}

	return nil
}

// scanPluginDirectory finds and loads plugins
func (p *Plugins) scanPluginDirectory(b *builtins.Builtins) error {
	if p.pluginDir == "" {
		return errors.New("invalid plugin directory (none)")
	}

	p.logger.Info().
		Str("dir", p.pluginDir).
		Msg("Scanning plugin directory")

	f, err := os.Open(p.pluginDir)
	if err != nil {
		return errors.Wrap(err, "open plugin directory")
	}

	defer f.Close()

	files, err := f.Readdir(-1)
	if err != nil {
		return errors.Wrap(err, "reading plugin directory")
	}

	ttlRx, err := regexp.Compile(`_ttl(.+)$`)
	if err != nil {
		return errors.Wrap(err, "compiling ttl regex")
	}
	ttlUnitRx, err := regexp.Compile(`(ms|s|m|h)$`)
	if err != nil {
		return errors.Wrap(err, "compiling ttl unit regex")
	}

	for _, fi := range files {
		fileName := fi.Name()

		p.logger.Debug().
			Str("path", filepath.Join(p.pluginDir, fileName)).
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
			cmdName = filepath.Join(p.pluginDir, fi.Name())
		case mode&os.ModeSymlink != 0:
			resolvedSymlink, err := filepath.EvalSymlinks(filepath.Join(p.pluginDir, fi.Name()))
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

		if b != nil && b.IsBuiltin(fileBase) {
			p.logger.Warn().Str("id", fileBase).Msg("Builtin collector already enabled, skipping plugin")
			continue
		}

		var cfg map[string][]string

		// check for config file
		cfgFile := filepath.Join(p.pluginDir, fmt.Sprintf("%s.json", fileBase))
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

		// parse fileBase for _ttl(.+)
		matches := ttlRx.FindAllStringSubmatch(fileBase, -1)
		var runTTL time.Duration
		if len(matches) > 0 && len(matches[0]) > 1 {
			ttl := matches[0][1]
			if ttl != "" {
				if !ttlUnitRx.MatchString(ttl) {
					ttl += viper.GetString(config.KeyPluginTTLUnits)
				}

				if d, err := time.ParseDuration(ttl); err != nil {
					p.logger.Warn().Err(err).Str("ttl", ttl).Msg("parsing plugin ttl, ignoring ttl")
				} else {
					runTTL = d
				}
			}
		}

		if cfg == nil {
			plug, ok := p.active[fileBase]
			if !ok {
				p.active[fileBase] = &plugin{
					ctx:    p.ctx,
					id:     fileBase,
					name:   fileBase,
					logger: p.logger.With().Str("plugin", fileBase).Logger(),
					runDir: p.pluginDir,
					runTTL: runTTL,
				}
				plug = p.active[fileBase]
			}

			appstats.MapIncrementInt("plugins", "total")
			plug.command = cmdName
			p.logger.Info().
				Str("id", fileBase).
				Str("cmd", cmdName).
				Msg("Activating plugin")

		} else {
			for inst, args := range cfg {
				pluginName := fmt.Sprintf("%s`%s", fileBase, inst)
				plug, ok := p.active[pluginName]
				if !ok {
					p.active[pluginName] = &plugin{
						ctx:          p.ctx,
						id:           fileBase,
						instanceID:   inst,
						instanceArgs: args,
						name:         pluginName,
						logger:       p.logger.With().Str("plugin", pluginName).Logger(),
						runDir:       p.pluginDir,
						runTTL:       runTTL,
					}
					plug = p.active[pluginName]
				}

				appstats.MapIncrementInt("plugins", "total")
				plug.command = cmdName
				p.logger.Info().
					Str("id", pluginName).
					Str("cmd", cmdName).
					Msg("Activating plugin")

			}
		}
	}

	if len(p.active) == 0 {
		return errors.New("No active plugins found")
	}

	return nil
}
