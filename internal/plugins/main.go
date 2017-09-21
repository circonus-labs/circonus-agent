// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package plugins

import (
	"encoding/json"
	"strings"
	"sync"

	// "github.com/rjeczalik/notify"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// PluginList all active plugins
type PluginList struct {
	sync.RWMutex
	generation uint64
	active     map[string]*Plugin
	running    bool
}

var (
	pluginList    *PluginList
	pluginDir     string
	logger        zerolog.Logger
	reservedNames = map[string]bool{"write": true, "statsd": true}
)

// Initialize the plugin manager
func Initialize() error {
	logger = log.With().Str("pkg", "plugins").Logger()

	pluginDir = viper.GetString(config.KeyPluginDir)
	pluginList = &PluginList{
		generation: 0,
		active:     make(map[string]*Plugin),
	}

	return Scan()

	// start plugin directory watcher
	// go pluginWatcher()
}

// Flush plugin metrics
func Flush(plugin string) map[string]interface{} {
	pluginList.RLock()
	defer pluginList.RUnlock()

	metrics := map[string]interface{}{}

	for pluginID, plug := range pluginList.active {
		if plugin == "" || // all plugins
			pluginID == plugin || // specific plugin
			strings.HasPrefix(pluginID, plugin+"`") { // specific plugin with instances
			metrics[pluginID] = plug.drain()
		}
	}

	return metrics
}

// Run one or all plugins
func Run(plugin string) error {
	pluginList.Lock()
	defer pluginList.Unlock()

	if pluginList.running {
		msg := "plugin run already in progress"
		logger.Info().Msg(msg)
		return errors.Errorf(msg)
	}

	pluginList.running = true

	var wg sync.WaitGroup

	if plugin != "" {
		numFound := 0
		for pluginID, plug := range pluginList.active {
			if pluginID == plugin || // specific plugin
				strings.HasPrefix(pluginID, plugin+"`") { // specific plugin with instances
				numFound++
				wg.Add(1)
				go func(id string, plug *Plugin) {
					plug.exec()
					wg.Done()
				}(pluginID, plug)
			}
		}
		if numFound == 0 {
			logger.Error().
				Str("plugin", plugin).
				Msg("Invalid/Unknown")
			pluginList.running = false
			return errors.Errorf("invalid plugin (%s)", plugin)
		}
	} else {
		wg.Add(len(pluginList.active))
		for pluginID, pluginRef := range pluginList.active {
			go func(id string, plug *Plugin) {
				plug.exec()
				wg.Done()
			}(pluginID, pluginRef)
		}
	}

	wg.Wait()
	logger.Debug().Msg("all plugins done")

	pluginList.running = false

	return nil
}

// IsValid determines if a specific plugin is valid
func IsValid(plugin string) bool {
	if plugin == "" {
		return false
	}

	pluginList.RLock()
	defer pluginList.RUnlock()

	for pluginID := range pluginList.active {
		// specific plugin       plugin with instances
		if pluginID == plugin || strings.HasPrefix(pluginID, plugin+"`") {
			return true
		}
	}

	return false
}

// IsInternal checks to see if the plugin is one of the internal plugins (write|statsd)
func IsInternal(plugin string) bool {
	if plugin == "" {
		return false
	}
	_, reserved := reservedNames[plugin]

	return reserved
}

// Inventory returns list of active plugins
func Inventory() ([]byte, error) {
	pluginList.RLock()
	defer pluginList.RUnlock()
	return json.Marshal(pluginList.active)
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
