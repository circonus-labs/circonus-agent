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
	"runtime"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	"github.com/maier/go-appstats"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// Scan the plugin directory for new/updated plugins
func (p *Plugins) Scan(b *builtins.Builtins) error {
	p.Lock()
	defer p.Unlock()

	// initialRun fires each plugin one time. Unlike 'Run' it does
	// not wait for plugins to finish this provides:
	//
	// 1. an initial seeding of results
	// 2. starts any long running plugins without blocking
	//
	initialRun := func() {
		for id, plug := range p.active {
			p.logger.Debug().
				Str("plugin", id).
				Msg("Initializing")
			go func(plug *plugin) {
				if err := plug.exec(); err != nil {
					p.logger.Error().Err(err).Msg("executing")
				}
			}(plug)
		}
	}

	pluginList := viper.GetStringSlice(config.KeyPluginList)

	if p.pluginDir != "" {
		if err := p.scanPluginDirectory(b); err != nil {
			return errors.Wrap(err, "plugin directory scan")
		}
	} else if len(pluginList) > 0 {
		if err := p.verifyPluginList(pluginList); err != nil {
			return errors.Wrap(err, "verifying plugin list")
		}
	}

	initialRun()

	if len(p.active) == 0 {
		p.logger.Warn().Msg("no active plugins found")
	}

	return nil
}

// verifyPluginList checks supplied list of plugin commands
func (p *Plugins) verifyPluginList(l []string) error {
	if len(l) == 0 {
		return errors.New("invalid plugin list (empty)")
	}

	ttlRx := regexp.MustCompile(`_ttl(.+)$`)
	ttlUnitRx := regexp.MustCompile(`(ms|s|m|h)$`)

	for _, fileSpec := range l {
		fileDir, fileName := filepath.Split(fileSpec)
		fileBase := fileName
		fileExt := filepath.Ext(fileName)

		if fileExt != "" {
			fileBase = strings.ReplaceAll(fileName, fileExt, "")
		}

		fs, err := os.Stat(fileSpec)
		if err != nil {
			p.logger.Warn().Err(err).Str("file", fileSpec).Msg("skipping")
			continue
		}
		if fs.IsDir() {
			p.logger.Warn().Str("file", fileSpec).Msg("directory, skipping")
		}

		var cmdName string

		switch mode := fs.Mode(); {
		case mode.IsRegular():
			cmdName = fileSpec
		case mode&os.ModeSymlink != 0:
			resolvedSymlink, err := filepath.EvalSymlinks(fileSpec)
			if err != nil {
				p.logger.Warn().
					Err(err).
					Str("file", fileSpec).
					Msg("error resolving symlink, ignoring")
				continue
			}
			cmdName = resolvedSymlink
		default:
			p.logger.Debug().
				Str("file", fileSpec).
				Msg("not a regular file or symlink, ignoring")
			continue // just ignore it
		}

		if runtime.GOOS != "windows" {
			// windows doesn't have an e'x'ecutable bit, all files are
			// 'potentially' executable - binary exe, interpreted scripts, etc.
			if perm := fs.Mode().Perm() & 0111; perm != 73 {
				p.logger.Warn().
					Str("file", fileSpec).
					Str("perms", fmt.Sprintf("%q", fs.Mode().Perm())).
					Msg("executable bit not set, ignoring")
				continue
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
					p.logger.Warn().Err(err).Str("file", fileSpec).Str("ttl", ttl).Msg("parsing plugin ttl, ignoring ttl")
				} else {
					runTTL = d
				}
			}
		}

		plug, ok := p.active[fileBase]
		if !ok {
			p.active[fileBase] = &plugin{
				ctx:      p.ctx,
				id:       fileBase,
				name:     fileBase,
				logger:   p.logger.With().Str("id", fileBase).Logger(),
				runDir:   fileDir,
				runTTL:   runTTL,
				baseTags: tags.GetBaseTags(),
			}
			plug = p.active[fileBase]
		}

		_ = appstats.IncrementInt("plugins.total")
		// appstats.MapIncrementInt("plugins", "total")
		plug.command = cmdName
		p.logger.Info().Str("id", fileBase).Str("cmd", cmdName).Msg("activating")
	}

	return nil
}

// scanPluginDirectory finds and loads plugins
func (p *Plugins) scanPluginDirectory(b *builtins.Builtins) error {
	if p.pluginDir == "" {
		return errors.New("invalid plugin directory (none)")
	}

	p.logger.Info().Str("dir", p.pluginDir).Msg("scanning")

	f, err := os.Open(p.pluginDir)
	if err != nil {
		return errors.Wrap(err, "open plugin directory")
	}

	defer f.Close()

	files, err := f.Readdir(-1)
	if err != nil {
		return errors.Wrap(err, "reading plugin directory")
	}

	ttlRx := regexp.MustCompile(`_ttl(.+)$`)
	ttlUnitRx := regexp.MustCompile(`(ms|s|m|h)$`)

	for _, fi := range files {
		fileName := fi.Name()

		// skip the README.md file placed in the default plugins
		// directory during installation. (it "appears" executable
		// on Windows).
		if strings.ToLower(fileName) == "readme.md" {
			continue
		}

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
			fileBase = strings.ReplaceAll(fileName, fileExt, "")
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
					Msg("error resolving symlink, ignoring")
				continue
			}
			cmdName = resolvedSymlink
		default:
			p.logger.Debug().
				Str("file", fileName).
				Msg("not a regular file or symlink, ignoring")
			continue // just ignore it
		}

		if runtime.GOOS != "windows" {
			// windows doesn't have an e'x'ecutable bit, all files are
			// 'potentially' executable - binary exe, interpreted scripts, etc.
			if perm := fi.Mode().Perm() & 0111; perm != 73 {
				p.logger.Warn().
					Str("file", cmdName).
					Str("perms", fmt.Sprintf("%q", fi.Mode().Perm())).
					Msg("executable bit not set, ignoring")
				continue
			}
		}

		if b != nil && b.IsBuiltin(fileBase) {
			p.logger.Warn().Str("id", fileBase).Msg("builtin collector already enabled, skipping plugin")
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
					ctx:      p.ctx,
					id:       fileBase,
					name:     fileBase,
					logger:   p.logger.With().Str("id", fileBase).Logger(),
					runDir:   p.pluginDir,
					runTTL:   runTTL,
					baseTags: tags.GetBaseTags(),
				}
				plug = p.active[fileBase]
			}

			_ = appstats.IncrementInt("plugins.total")
			// appstats.MapIncrementInt("plugins", "total")
			plug.command = cmdName
			p.logger.Info().Str("id", fileBase).Str("cmd", cmdName).Msg("activating")

		} else {
			for inst, args := range cfg {
				pluginName := fileBase + defaults.MetricNameSeparator + inst
				plug, ok := p.active[pluginName]
				if !ok {
					p.active[pluginName] = &plugin{
						ctx:          p.ctx,
						id:           fileBase,
						instanceID:   inst,
						instanceArgs: args,
						name:         pluginName,
						logger:       p.logger.With().Str("id", pluginName).Logger(),
						runDir:       p.pluginDir,
						runTTL:       runTTL,
						baseTags:     tags.GetBaseTags(),
					}
					plug = p.active[pluginName]
				}

				_ = appstats.IncrementInt("plugins.total")
				// appstats.MapIncrementInt("plugins", "total")
				plug.command = cmdName
				p.logger.Info().Str("id", pluginName).Str("cmd", cmdName).Msg("activating")

			}
		}
	}

	return nil
}
