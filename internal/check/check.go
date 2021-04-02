// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package check handles check and broker management
package check

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/check/bundle"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/go-apiclient"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// Check exposes the check bundle management interface.
type Check struct {
	checkConfig           *apiclient.Check
	checkBundle           *bundle.Bundle
	broker                *apiclient.Broker
	statusActiveBroker    string
	client                API
	revConfigs            *ReverseConfigs
	logger                zerolog.Logger
	brokerMaxResponseTime time.Duration
	refreshTTL            time.Duration
	brokerMaxRetries      int
	reverse               bool
	sync.Mutex
}

// Meta contains check id meta data.
type Meta struct {
	BundleID  string
	CheckUUID string
	CheckID   string
}

// ReverseConfig contains the reverse configuration for the check.
type ReverseConfig struct {
	BrokerAddr *net.TCPAddr
	ReverseURL *url.URL
	TLSConfig  *tls.Config
	CN         string
	BrokerID   string
}

type ReverseConfigs map[string]ReverseConfig

const (
	StatusActive      = "active"
	PrimaryCheckIndex = 0
)

var (
	errCheckNotInitialized = fmt.Errorf("check not initialized")
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
		s = s + " Bundle: " + e.BundleID
	}
	if e.CheckID != "" {
		s = s + " Check: " + e.CheckID
	}
	return s
}

func (e *ErrNoOwnerFound) Error() string {
	if e == nil {
		return "<nil>"
	}
	s := e.Err
	if e.CheckID != "" {
		s = s + " Check: " + e.CheckID
	}
	return s
}

func (e *ErrInvalidOwner) Error() string {
	if e == nil {
		return "<nil>"
	}
	s := e.Err
	if e.CheckID != "" {
		s = s + " Check: " + e.CheckID
	}
	if e.BrokerCN != "" {
		s = s + " CN: " + e.BrokerCN
	}
	return s
}

// logshim is used to satisfy apiclient Logger interface (avoiding ptr receiver issue).
type logshim struct {
	logh zerolog.Logger
}

func (l logshim) Printf(fmt string, v ...interface{}) {
	l.logh.Printf(fmt, v...)
}

// New returns a new check instance.
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
	isReverse := viper.GetBool(config.KeyReverse)
	cid := viper.GetString(config.KeyCheckBundleID)
	needCheck := false

	if isReverse || (isCreate && cid == "") {
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
		if caFile := viper.GetString(config.KeyAPICAFile); caFile != "" {
			cfg.CACert = c.loadAPICAfile(caFile)
		}
		client, err := apiclient.New(cfg)
		if err != nil {
			return nil, fmt.Errorf("creating circonus api client: %w", err)
		}
		apiClient = client
	}

	c.client = apiClient

	b, err := bundle.New(c.client)
	if err != nil {
		return nil, fmt.Errorf("new bundle: %w", err)
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
			return nil, fmt.Errorf("setting up reverse configuration: %w", err)
		}
		c.reverse = true
	}

	return &c, nil
}

// CheckMeta returns check id, check bundle id, and check uuid.
func (c *Check) CheckMeta() (*Meta, error) {
	c.Lock()
	defer c.Unlock()

	if c.checkConfig == nil {
		return nil, errCheckNotInitialized
	}

	return &Meta{
		BundleID:  c.checkConfig.CheckBundleCID,
		CheckID:   c.checkConfig.CID,
		CheckUUID: c.checkConfig.CheckUUID,
	}, nil
}

// SubmissionURL returns the URL to submit metrics to as well as the tls config for https.
func (c *Check) SubmissionURL() (string, *tls.Config, error) {
	surl, err := c.checkBundle.SubmissionURL()
	if err != nil {
		return "", nil, fmt.Errorf("submission url: %w", err)
	}

	u, err := url.Parse(surl)
	if err != nil {
		return "", nil, fmt.Errorf("parsing submission url: %w", err)
	}
	tlsConfig, _, err := c.brokerTLSConfig(u)
	if err != nil {
		return "", nil, fmt.Errorf("creating TLS config for (%s - %s): %w", c.broker.CID, surl, err)
	}

	return surl, tlsConfig, nil
}

// CheckPeriod returns check bundle period (intetrval between when broker should make request).
func (c *Check) CheckPeriod() (uint, error) {
	c.Lock()
	defer c.Unlock()

	if c.checkBundle == nil {
		return 0, errCheckNotInitialized
	}

	period, err := c.checkBundle.Period()
	if err != nil {
		return 0, fmt.Errorf("check period: %w", err)
	}

	return period, nil
}

// RefreshReverseConfig refreshes the check, broker and broker tls configurations.
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

// GetReverseConfigs returns the reverse connection configuration(s) to use for the check.
func (c *Check) GetReverseConfigs() (*ReverseConfigs, error) {
	c.Lock()
	defer c.Unlock()

	if !c.reverse {
		return nil, fmt.Errorf("agent not in reverse mode") //nolint:goerr113
	}

	if c.revConfigs == nil {
		return nil, fmt.Errorf("invalid reverse config (nil)") //nolint:goerr113
	}

	return c.revConfigs, nil
}

// FetchCheckConfig re-loads the check using the API.
func (c *Check) FetchCheckConfig() error {
	c.Lock()
	defer c.Unlock()

	if c.checkBundle == nil {
		return errCheckNotInitialized
	}

	checkCID, err := c.checkBundle.CheckCID(PrimaryCheckIndex)
	if err != nil {
		return fmt.Errorf("check cid: %w", err)
	}

	check, err := c.client.FetchCheck(apiclient.CIDType(&checkCID))
	if err != nil {
		return fmt.Errorf("unable to fetch check (%s): %w", checkCID, err)
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

// FetchBrokerConfig re-loads the broker using the API.
func (c *Check) FetchBrokerConfig() error {
	c.Lock()
	defer c.Unlock()

	if c.checkConfig == nil {
		return errCheckNotInitialized
	}

	broker, err := c.client.FetchBroker(apiclient.CIDType(&c.checkConfig.BrokerCID))
	if err != nil {
		return fmt.Errorf("unable to fetch broker (%s): %w", c.checkConfig.BrokerCID, err)
	}

	c.broker = broker
	c.logger.Debug().Interface("config", c.broker).Msg("using broker config")

	return nil
}

func (c *Check) loadAPICAfile(file string) *x509.CertPool {
	cp := x509.NewCertPool()
	cert, err := ioutil.ReadFile(file)
	if err != nil {
		c.logger.Error().Err(err).Str("file", file).Msg("unable to load api ca file")
		return nil
	}
	if !cp.AppendCertsFromPEM(cert) {
		c.logger.Error().Err(err).Str("file", file).Msg("problem parsing cert in api ca file")
		return nil
	}
	return cp
}
