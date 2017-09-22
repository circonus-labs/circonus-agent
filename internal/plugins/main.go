// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package plugins

import (
	"encoding/json"
	"os/exec"
	"strings"
	"sync"

	// "github.com/rjeczalik/notify"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// Metric defines an individual metric sample or array of samples (histogram)
type Metric struct {
	Type  string      `json:"_type"`
	Value interface{} `json:"_value"`
}

// Metrics defines the list of metrics for a given plugin
type Metrics map[string]Metric

// Plugin defines a specific plugin
type plugin struct {
	sync.RWMutex
	cmd          *exec.Cmd
	metrics      *Metrics
	prevMetrics  *Metrics
	logger       zerolog.Logger
	ID           string
	InstanceID   string
	Name         string
	InstanceArgs []string
	Command      string
	Generation   uint64
	Running      bool
	RunDir       string
}

// Plugins defines plugin manager
type Plugins struct {
	sync.RWMutex
	generation    uint64
	active        map[string]*plugin
	running       bool
	pluginDir     string
	logger        zerolog.Logger
	reservedNames map[string]bool
}

const (
	metricDelimiter = "`"
	fieldDelimiter  = "\t"
	nullMetricValue = "[[null]]"
)

// New returns a new instance of the plugins manager
func New() *Plugins {
	p := Plugins{
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

// Inventory returns list of active plugins
func (p *Plugins) Inventory() ([]byte, error) {
	p.RLock()
	defer p.RUnlock()
	return json.Marshal(p.active)
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
