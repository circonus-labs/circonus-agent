// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package check handles check and broker management
package check

import (
	"crypto/tls"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/check/bundle"
	"github.com/circonus-labs/circonus-agent/internal/config"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/go-apiclient"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// Check exposes the check bundle management interface
type Check struct {
	statusActiveBroker    string
	brokerMaxResponseTime time.Duration
	brokerMaxRetries      int
	checkConfig           *apiclient.Check
	checkBundle           *bundle.Bundle
	broker                *apiclient.Broker
	client                API
	logger                zerolog.Logger
	refreshTTL            time.Duration
	reverse               bool
	revConfigs            *ReverseConfigs
	sync.Mutex
}

// Meta contains check id meta data
type Meta struct {
	BundleID  string
	CheckUUID string
	CheckID   string
}

// ReverseConfig contains the reverse configuration for the check
type ReverseConfig struct {
	CN         string
	BrokerAddr *net.TCPAddr
	BrokerID   string
	ReverseURL *url.URL
	TLSConfig  *tls.Config
}

type ReverseConfigs map[string]ReverseConfig

const (
	StatusActive      = "active"
	PrimaryCheckIndex = 0
)

type ErrNoOwnerFound struct {
	Err     string
	CheckID string
}

type ErrInvalidOwner struct {
	Err      string
	CheckID  string
	BrokerCN string
}

type ErrNotActive struct {
	Err      string
	CheckID  string
	BundleID string
}

func (e *ErrNotActive) Error() string {
	if e == nil {
		return "<nil>"
	}
	s := e.Err
	if e.BundleID != "" {
		s = s + "Bundle: " + e.BundleID + " "
	}
	if e.CheckID != "" {
		s = s + "Check: " + e.CheckID + " "
	}
	return s
}

func (e *ErrNoOwnerFound) Error() string {
	if e == nil {
		return "<nil>"
	}
	s := e.Err
	if e.CheckID != "" {
		s = s + "Check: " + e.CheckID + " "
	}
	return s
}

func (e *ErrInvalidOwner) Error() string {
	if e == nil {
		return "<nil>"
	}
	s := e.Err
	if e.CheckID != "" {
		s = s + "Check: " + e.CheckID + " "
	}
	if e.BrokerCN != "" {
		s = s + "CN: " + e.BrokerCN + " "
	}
	return s
}

// logshim is used to satisfy apiclient Logger interface (avoiding ptr receiver issue)
type logshim struct {
	logh zerolog.Logger
}

func (l logshim) Printf(fmt string, v ...interface{}) {
	l.logh.Printf(fmt, v...)
}

// New returns a new check instance
func New(apiClient API) (*Check, error) {
	// NOTE: TBD, make broker max retries and response time configurable
	c := Check{
		brokerMaxResponseTime: 500 * time.Millisecond,
		brokerMaxRetries:      5,
		checkConfig:           nil,
		checkBundle:           nil,
		broker:                nil,
		logger:                log.With().Str("pkg", "check").Logger(),
		refreshTTL:            time.Duration(0),
		reverse:               false,
		statusActiveBroker:    StatusActive,
	}

	isCreate := viper.GetBool(config.KeyCheckCreate)
	isManaged := viper.GetBool(config.KeyCheckEnableNewMetrics)
	isReverse := viper.GetBool(config.KeyReverse)
	cid := viper.GetString(config.KeyCheckBundleID)
	needCheck := false

	if isReverse || isManaged || (isCreate && cid == "") {
		needCheck = true
	}

	if !needCheck {
		c.logger.Info().Msg("check management disabled")
		return &c, nil // if we don't need a check, return a NOP object
	}

	if apiClient == nil {
		// create an API client
		cfg := &apiclient.Config{
			Debug:    viper.GetBool(config.KeyDebugAPI),
			Log:      logshim{logh: c.logger.With().Str("pkg", "circ.api").Logger()},
			TokenApp: viper.GetString(config.KeyAPITokenApp),
			TokenKey: viper.GetString(config.KeyAPITokenKey),
			URL:      viper.GetString(config.KeyAPIURL),
		}
		client, err := apiclient.New(cfg)
		if err != nil {
			return nil, errors.Wrap(err, "creating circonus api client")
		}
		apiClient = client
	}

	c.client = apiClient

	b, err := bundle.New(c.client)
	if err != nil {
		return nil, err
	}

	c.checkBundle = b

	if err := c.FetchCheckConfig(); err != nil {
		return nil, err
	}

	if err := c.FetchBrokerConfig(); err != nil {
		return nil, err
	}

	if isReverse {
		err := c.setReverseConfigs()
		if err != nil {
			return nil, errors.Wrap(err, "setting up reverse configuration")
		}
		c.reverse = true
	}

	return &c, nil
}

// CheckMeta returns check id, check bundle id, and check uuid
func (c *Check) CheckMeta() (*Meta, error) {
	c.Lock()
	defer c.Unlock()

	if c.checkConfig == nil {
		return nil, errors.New("check uninitialized")
	}

	return &Meta{
		BundleID:  c.checkConfig.CheckBundleCID,
		CheckID:   c.checkConfig.CID,
		CheckUUID: c.checkConfig.CheckUUID,
	}, nil
}

// CheckPeriod returns check bundle period (intetrval between when broker should make request)
func (c *Check) CheckPeriod() (uint, error) {
	c.Lock()
	defer c.Unlock()

	if c.checkBundle == nil {
		return 0, errors.New("check bundle uninitialized")
	}

	return c.checkBundle.Period()
}

// RefreshReverseConfig refreshes the check, broker and broker tls configurations
func (c *Check) RefreshReverseConfig() error {
	if err := c.FetchCheckConfig(); err != nil {
		return err
	}
	if err := c.FetchBrokerConfig(); err != nil {
		return err
	}
	if err := c.setReverseConfigs(); err != nil {
		return err
	}
	return nil
}

// GetReverseConfigs returns the reverse connection configuration(s) to use for the check
func (c *Check) GetReverseConfigs() (*ReverseConfigs, error) {
	c.Lock()
	defer c.Unlock()

	if !c.reverse {
		return nil, errors.New("agent not in reverse mode")
	}

	if c.revConfigs == nil {
		return nil, errors.New("invalid reverse config (nil)")
	}

	return c.revConfigs, nil
}

// FetchCheckConfig re-loads the check using the API
func (c *Check) FetchCheckConfig() error {
	c.Lock()
	defer c.Unlock()

	if c.checkBundle == nil {
		return errors.New("check bundle uninitialized")
	}

	checkCID, err := c.checkBundle.CheckCID(PrimaryCheckIndex)
	if err != nil {
		return err
	}

	check, err := c.client.FetchCheck(apiclient.CIDType(&checkCID))
	if err != nil {
		return errors.Wrapf(err, "unable to fetch check (%s)", checkCID)
	}

	if !check.Active {
		return &ErrNotActive{
			Err:      "check is not active",
			BundleID: check.CheckBundleCID,
			CheckID:  check.CID,
		}
	}

	c.checkConfig = check
	c.logger.Debug().Interface("config", c.checkConfig).Msg("using check config")

	return nil
}

// FetchBrokerConfig re-loads the broker using the API
func (c *Check) FetchBrokerConfig() error {
	c.Lock()
	defer c.Unlock()

	if c.checkConfig == nil {
		return errors.New("check uninitialized")
	}

	broker, err := c.client.FetchBroker(apiclient.CIDType(&c.checkConfig.BrokerCID))
	if err != nil {
		return errors.Wrapf(err, "unable to fetch broker (%s)", c.checkConfig.BrokerCID)
	}

	c.broker = broker
	c.logger.Debug().Interface("config", c.broker).Msg("using broker config")

	return nil
}

// EnableNewMetrics updates the check bundle enabling any new metrics
func (c *Check) EnableNewMetrics(m *cgm.Metrics) error {
	if c.checkBundle == nil {
		return nil // noop -- errors.New("check bundle uninitialized")
	}

	return c.checkBundle.EnableNewMetrics(m)
}
