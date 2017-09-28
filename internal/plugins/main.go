// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package plugins

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	// "github.com/rjeczalik/notify"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// New returns a new instance of the plugins manager
func New(ctx context.Context) *Plugins {
	p := Plugins{
		ctx:           ctx,
		generation:    0,
		running:       false,
		pluginDir:     viper.GetString(config.KeyPluginDir),
		logger:        log.With().Str("pkg", "plugins").Logger(),
		reservedNames: map[string]bool{"write": true, "statsd": true},
		active:        make(map[string]*plugin),
	}

	return &p
}

// Flush plugin metrics
func (p *Plugins) Flush(pluginName string) *map[string]interface{} {
	p.RLock()
	defer p.RUnlock()

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

	// for id, plug := range p.active {
	// 	plug.Lock()
	// 	if !plug.Running {
	// 		plug.Unlock()
	// 		continue
	// 	}
	// 	if plug.cmd == nil {
	// 		plug.Unlock()
	// 		continue
	// 	}
	// 	if plug.cmd.Process != nil {
	// 		var stop bool
	// 		if plug.cmd.ProcessState == nil {
	// 			stop = true
	// 		} else {
	// 			stop = !plug.cmd.ProcessState.Exited()
	// 		}
	//
	// 		if stop {
	// 			p.logger.Info().
	// 				Str("plugin", id).
	// 				Msg("Stopping running plugin")
	// 			err := plug.cmd.Process.Kill()
	// 			if err != nil {
	// 				p.logger.Error().
	// 					Err(err).
	// 					Str("plugin", id).
	// 					Msg("Stopping plugin")
	// 			}
	// 		}
	// 	}
	// 	plug.Unlock()
	// }
	// return nil
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

	p.running = true

	var wg sync.WaitGroup

	if pluginName != "" {
		numFound := 0
		for pluginID, plug := range p.active {
			if pluginID == pluginName || // specific plugin
				strings.HasPrefix(pluginID, pluginName+"`") { // specific plugin with instances
				numFound++
				wg.Add(1)
				go func(id string, plug *plugin) {
					plug.exec()
					wg.Done()
				}(pluginID, plug)
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
		wg.Add(len(p.active))
		for pluginID, pluginRef := range p.active {
			go func(id string, plug *plugin) {
				plug.exec()
				wg.Done()
			}(pluginID, pluginRef)
		}
	}

	wg.Wait()
	p.logger.Debug().Msg("all plugins done")

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

type lastRunError struct {
	Code int    `json:"code"`
	Msg  string `json:"message"`
}

type pluginDetails struct {
	Name            string   `json:"name"`
	Instance        string   `json:"instance"`
	Command         string   `json:"command"`
	Args            []string `json:"args"`
	LastRunStart    string   `json:"last_run_start"`
	LastRunDuration string   `json:"last_run_duration"`
	LastError       string   `json:"last_error"`
}

// Inventory returns list of active plugins
func (p *Plugins) Inventory() []byte {
	p.Lock()
	defer p.Unlock()
	inventory := make(map[string]*pluginDetails, len(p.active))
	for id, plug := range p.active {
		plug.Lock()
		inventory[id] = &pluginDetails{
			Name:            plug.ID,
			Instance:        plug.InstanceID,
			Command:         plug.Command,
			Args:            plug.InstanceArgs,
			LastRunStart:    plug.LastStart.Format(time.RFC3339Nano),
			LastRunDuration: plug.LastRunDuration.String(),
		}

		if plug.LastError != nil {
			inventory[id].LastError = plug.LastError.Error()
		}

		plug.Unlock()
	}
	data, err := json.Marshal(inventory)
	if err != nil {
		p.logger.Fatal().Err(err).Msg("inventory -> json")
	}
	return data
}

// func pluginWatcher() {
// 	c := make(chan notify.EventInfo, 1)
//
// 	if err := notify.Watch(pluginDir, c, notify.All); err != nil {
// 		logger.Fatal().
// 			Err(err).
// 			Str("plugin-dir", pluginDir).
// 			Msg("Unable to watch plugin directory")
// 	}
//
// 	defer notify.Stop(c)
//
// 	for ei := range c {
// 		logger.Debug().
// 			Str("event", ei.Event().String()).
// 			Str("path", ei.Path()).
// 			Msg("event")
// 	}
// }
