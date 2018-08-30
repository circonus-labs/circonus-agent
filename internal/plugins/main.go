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

	"github.com/circonus-labs/circonus-agent/api"
	"github.com/circonus-labs/circonus-agent/internal/config"
	cgm "github.com/circonus-labs/circonus-gometrics"
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
		reservedNames: map[string]bool{"prom": true, "write": true, "statsd": true},
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
func (p *Plugins) Flush(pluginName string) *cgm.Metrics {
	p.RLock()
	defer p.RUnlock()

	appstats.SetString("plugins.last_flush", time.Now().String())
	// appstats.MapSet("plugins", "last_flush", time.Now())

	metrics := cgm.Metrics{}

	for pluginID, plug := range p.active {
		if pluginName == "" || // all plugins
			pluginID == pluginName || // specific plugin
			strings.HasPrefix(pluginID, pluginName+metricDelimiter) { // specific plugin with instances

			m := plug.drain()
			for mn, mv := range *m {
				metrics[pluginID+metricDelimiter+mn] = mv
			}
		}
	}

	return &metrics
}

// Stop any long running plugins
func (p *Plugins) Stop() error {
	p.logger.Info().Msg("stopping")
	return nil
}

// Run one or all plugins
func (p *Plugins) Run(pluginName string) error {
	p.Lock()

	if p.running {
		msg := "plugin run already in progress"
		p.logger.Info().Msg(msg)
		p.Unlock()
		return errors.Errorf(msg)
	}

	start := time.Now()
	appstats.SetString("plugins.last_run_start", start.String())
	// appstats.MapSet("plugins", "last_run_start", start)

	p.running = true
	p.Unlock()

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
			p.logger.Error().Str("plugin", pluginName).Msg("invalid/unknown")
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

	appstats.SetString("plugins.last_run_end", time.Now().String())
	appstats.SetString("plugins.last_run_duration", time.Since(start).String())
	// appstats.MapSet("plugins", "last_run_end", time.Now())
	// appstats.MapSet("plugins", "last_run_duration", time.Since(start))

	p.Lock()
	p.running = false
	p.logger.Debug().Msg("all plugins done")
	p.Unlock()

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
	inventory := api.Inventory{}
	for id, plug := range p.active {
		plug.Lock()
		pinfo := api.Plugin{
			ID:              id,
			Name:            plug.id,
			Instance:        plug.instanceID,
			Command:         plug.command,
			Args:            plug.instanceArgs,
			LastRunStart:    plug.lastStart.Format(time.RFC3339Nano),
			LastRunEnd:      plug.lastEnd.Format(time.RFC3339Nano),
			LastRunDuration: plug.lastRunDuration.String(),
		}
		if plug.lastError != nil {
			pinfo.LastError = plug.lastError.Error()
		}
		plug.Unlock()
		inventory = append(inventory, pinfo)
	}
	data, err := json.Marshal(inventory)
	if err != nil {
		p.logger.Fatal().Err(err).Msg("inventory -> json")
	}
	return data
}
