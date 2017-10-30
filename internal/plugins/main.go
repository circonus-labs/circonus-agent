// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package plugins

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/maier/go-appstats"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// New returns a new instance of the plugins manager
func New(ctx context.Context) (*Plugins, error) {
	p := Plugins{
		ctx:           ctx,
		running:       false,
		logger:        log.With().Str("pkg", "plugins").Logger(),
		reservedNames: map[string]bool{"write": true, "statsd": true},
		active:        make(map[string]*plugin),
	}

	errMsg := "Invalid plugin directory"

	pluginDir := viper.GetString(config.KeyPluginDir)

	if pluginDir == "" {
		return nil, errors.New(errMsg + " (none)")
	}

	absDir, err := filepath.Abs(pluginDir)
	if err != nil {
		return nil, errors.Wrap(err, errMsg)
	}

	pluginDir = absDir

	fi, err := os.Stat(pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			p.logger.Warn().Err(err).Str("path", pluginDir).Msg("not found, ignoring")
			p.pluginDir = ""
			return &p, nil
		}
		return nil, errors.Wrap(err, errMsg)
	}

	if !fi.Mode().IsDir() {
		return nil, errors.Errorf(errMsg+" (%s) not a directory", pluginDir)
	}

	// also try opening, to verify permissions
	// if last dir on path is not accessible to user, stat doesn't return EPERM
	f, err := os.Open(pluginDir)
	if err != nil {
		return nil, errors.Wrap(err, errMsg)
	}
	f.Close()

	p.pluginDir = pluginDir

	return &p, nil
}

// Flush plugin metrics
func (p *Plugins) Flush(pluginName string) *map[string]interface{} {
	p.RLock()
	defer p.RUnlock()

	appstats.MapSet("plugins", "last_flush", time.Now())

	metrics := map[string]interface{}{}

	for pluginID, plug := range p.active {
		if pluginName == "" || // all plugins
			pluginID == pluginName || // specific plugin
			strings.HasPrefix(pluginID, pluginName+"`") { // specific plugin with instances
			metrics[pluginID] = plug.drain()
		}
	}

	return &metrics
}

// Stop any long running plugins
func (p *Plugins) Stop() error {
	p.logger.Info().Msg("Stopping plugins")
	return nil
}

// Run one or all plugins
func (p *Plugins) Run(pluginName string) error {
	p.Lock()
	defer p.Unlock()

	if p.running {
		msg := "plugin run already in progress"
		p.logger.Info().Msg(msg)
		return errors.Errorf(msg)
	}

	start := time.Now()
	appstats.MapSet("plugins", "last_run_start", start)

	p.running = true

	var wg sync.WaitGroup

	if pluginName != "" {
		numFound := 0
		for pluginID, pluginRef := range p.active {
			if pluginID == pluginName || // specific plugin
				strings.HasPrefix(pluginID, pluginName+"`") { // specific plugin with instances
				numFound++
				wg.Add(1)
				go func(id string, plug *plugin) {
					plug.exec()
					wg.Done()
				}(pluginID, pluginRef)
			}
		}
		if numFound == 0 {
			p.logger.Error().
				Str("plugin", pluginName).
				Msg("Invalid/Unknown")
			p.running = false
			return errors.Errorf("invalid plugin (%s)", pluginName)
		}
	} else {
		for pluginID, pluginRef := range p.active {
			wg.Add(1)
			go func(id string, plug *plugin) {
				plug.exec()
				wg.Done()
			}(pluginID, pluginRef)
		}
	}

	wg.Wait()
	p.logger.Debug().Msg("all plugins done")

	appstats.MapSet("plugins", "last_run_end", time.Now())
	appstats.MapSet("plugins", "last_run_duration", time.Since(start))

	p.running = false

	return nil
}

// IsValid determines if a specific plugin is valid
func (p *Plugins) IsValid(pluginName string) bool {
	if pluginName == "" {
		return false
	}

	p.RLock()
	defer p.RUnlock()

	for pluginID := range p.active {
		// specific plugin       plugin with instances
		if pluginID == pluginName || strings.HasPrefix(pluginID, pluginName+"`") {
			return true
		}
	}

	return false
}

// IsInternal checks to see if the plugin is one of the internal plugins (write|statsd)
func (p *Plugins) IsInternal(pluginName string) bool {
	if pluginName == "" {
		return false
	}
	_, reserved := p.reservedNames[pluginName]

	return reserved
}

// Inventory returns list of active plugins
func (p *Plugins) Inventory() []byte {
	p.Lock()
	defer p.Unlock()
	inventory := make(map[string]*pluginDetails, len(p.active))
	for id, plug := range p.active {
		plug.Lock()
		inventory[id] = &pluginDetails{
			Name:            plug.id,
			Instance:        plug.instanceID,
			Command:         plug.command,
			Args:            plug.instanceArgs,
			LastRunStart:    plug.lastStart.Format(time.RFC3339Nano),
			LastRunEnd:      plug.lastEnd.Format(time.RFC3339Nano),
			LastRunDuration: plug.lastRunDuration.String(),
		}

		if plug.lastError != nil {
			inventory[id].LastError = plug.lastError.Error()
		}

		plug.Unlock()
	}
	data, err := json.Marshal(inventory)
	if err != nil {
		p.logger.Fatal().Err(err).Msg("inventory -> json")
	}
	return data
}
