// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package bundle

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/circonus-labs/go-apiclient"
	apiconf "github.com/circonus-labs/go-apiclient/config"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// Bundle exposes the check bundle management interface
type Bundle struct {
	statusActiveMetric    string
	statusActiveBroker    string
	brokerMaxResponseTime time.Duration
	brokerMaxRetries      int
	bundle                *apiclient.CheckBundle
	client                API
	lastRefresh           time.Time
	logger                zerolog.Logger
	manage                bool
	metricStates          *metricStates
	metricStateUpdate     bool
	refreshTTL            time.Duration
	stateFile             string
	statePath             string
	sync.Mutex
}

var ErrUninitialized = errors.New("uninitialized check bundle")

type ErrNotActive struct {
	Err      string
	Checks   string
	BundleID string
	Status   string
}

func (e *ErrNotActive) Error() string {
	if e == nil {
		return "<nil>"
	}
	s := e.Err
	if e.BundleID != "" {
		s = s + "Bundle: " + e.BundleID + " "
	}
	if e.Checks != "" {
		s = s + "Check(s): " + e.Checks + " "
	}
	if e.Status != "" {
		s = s + "(" + e.Status + ")"
	}
	return s
}

const (
	StatusActive = "active"
)

func New(client API) (*Bundle, error) {

	if client == nil {
		return nil, errors.New("invalid client (nil)")
	}

	cb := Bundle{
		client:                client,
		brokerMaxResponseTime: 500 * time.Millisecond,
		brokerMaxRetries:      5,
		bundle:                nil,
		logger:                log.With().Str("pkg", "bundle").Logger(),
		manage:                false,
		metricStateUpdate:     false,
		refreshTTL:            time.Duration(0),
		statePath:             viper.GetString(config.KeyCheckMetricStateDir),
		statusActiveBroker:    StatusActive,
		statusActiveMetric:    StatusActive,
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
		cb.logger.Info().Msg("check management disabled")
		return &cb, nil // if we don't need a check, return a NOP object
	}

	// initialize the check bundle
	if err := cb.initCheckBundle(cid, isCreate); err != nil {
		return nil, errors.Wrap(err, "initializing check bundle")
	}

	// ensure a) the global check bundle id is set and b) it is set correctly
	// to the check bundle actually being used - need to do this even if the
	// check was created initially since user 'nobody' cannot create or update
	// the configuration (if one was used)
	viper.Set(config.KeyCheckBundleID, cb.bundle.CID)
	cb.logger.Debug().Interface("config", cb.bundle).Msg("using check bundle config")

	if isManaged && len(cb.bundle.MetricFilters) > 0 {
		cb.logger.Debug().Msg("disabling metric management, check bundle using metric_filters")
		isManaged = false
	}
	if !isManaged {
		cb.manage = false
		return &cb, nil
	}

	{
		//
		// NOTE: for metric management only
		//       metric management is DEPRECATED - this is for backwards compatibility
		//       and will be removed at some point in the future. all checks will be
		//       using metric filters going forward.
		//
		cb.stateFile = filepath.Join(cb.statePath, "metrics.json")

		if ok, err := cb.verifyStatePath(); !ok {
			if err != nil {
				cb.logger.Error().Err(err).Msg("verify state path")
			}
			cb.logger.Warn().Str("state_path", cb.statePath).Msg("encountered state path issue(s), disabling check-enable-new-metrics")
			viper.Set(config.KeyCheckEnableNewMetrics, false)
			cb.manage = false
			return &cb, nil
		}

		if ms, err := cb.loadState(); err != nil {
			cb.logger.Error().Err(err).Msg("unable to load existing metric states, all metrics considered existing")
		} else {
			cb.metricStates = ms
			cb.logger.Debug().Interface("metric_states", len(*cb.metricStates)).Msg("loaded metric states")
		}

		if err := cb.setMetricStates(&cb.bundle.Metrics); err != nil {
			return nil, errors.Wrap(err, "setting metric states")
		}
		cb.bundle.Metrics = []apiclient.CheckBundleMetric{} // save a little memory (or a lot depending on how many metrics are being managed...)
	}

	// check metrics refresh ttl
	ttl, err := time.ParseDuration(viper.GetString(config.KeyCheckMetricRefreshTTL))
	if err != nil {
		return nil, errors.Wrap(err, "parsing check metric refresh TTL")
	}
	if ttl == time.Duration(0) {
		ttl, err = time.ParseDuration(defaults.CheckMetricRefreshTTL)
		if err != nil {
			return nil, errors.Wrap(err, "parsing default check metric refresh TTL")
		}
	}

	cb.refreshTTL = ttl
	cb.manage = isManaged

	return &cb, nil
}

// CID returns the check bundle cid
func (cb *Bundle) CID() (string, error) {
	cb.Lock()
	defer cb.Unlock()

	if cb.bundle != nil {
		return cb.bundle.CID, nil
	}

	return "", ErrUninitialized
}

// Period returns check bundle period (intetrval between when broker should make requests)
func (cb *Bundle) Period() (uint, error) {
	cb.Lock()
	defer cb.Unlock()

	if cb.bundle != nil {
		return cb.bundle.Period, nil
	}

	return 0, ErrUninitialized
}

// Refresh re-loads the check bundle using the API (sets metric states if check bundle is managed)
func (cb *Bundle) Refresh() error {
	cb.Lock()
	defer cb.Unlock()

	if cb.bundle == nil {
		return ErrUninitialized
	}

	cb.logger.Debug().Msg("refreshing check configuration using API")

	b, err := cb.fetchCheckBundle(viper.GetString(config.KeyCheckBundleID))
	if err != nil {
		return errors.Wrap(err, "refresh check, fetching check")
	}

	cb.bundle = b

	if cb.manage {
		cb.logger.Debug().Msg("setting metric states")
		err := cb.setMetricStates(&cb.bundle.Metrics)
		if err != nil {
			return errors.Wrap(err, "setting metric states")
		}
	}
	cb.bundle.Metrics = []apiclient.CheckBundleMetric{} // save a little memory (or a lot depending on how many metrics are being managed...)

	return nil
}

// CheckCID returns the check cid at the passed index within the check bundle's checks array or an error if bundle not initialized
func (cb *Bundle) CheckCID(idx uint) (string, error) {
	cb.Lock()
	defer cb.Unlock()

	if cb.bundle == nil {
		return "", ErrUninitialized
	}
	if len(cb.bundle.Checks) == 0 {
		return "", errors.New("no checks found in check bundle")
	}
	if int(idx) > len(cb.bundle.Checks) {
		return "", errors.Errorf("invalid check index (%d>%d)", idx, len(cb.bundle.Checks))
	}

	return cb.bundle.Checks[idx], nil
}

// initCheck initializes the check for the agent.
// 1. fetch a check explicitly provided via CID
// 2. search for a check matching the current system
// 3. create a check for the system if --check-create specified
// if fetched, found, or created - set Check.bundle
// otherwise, return an error
func (cb *Bundle) initCheckBundle(cid string, create bool) error {
	var bundle *apiclient.CheckBundle

	// if explicit cid configured, attempt to fetch check bundle using cid
	if cid != "" {
		b, err := cb.fetchCheckBundle(cid)
		if err != nil {
			return errors.Wrapf(err, "fetching check for cid %s", cid)
		}
		bundle = b
	} else {
		// if no cid configured, attempt to find check bundle matching this system
		b, found, err := cb.findCheckBundle()
		if err != nil {
			if !create || found != 0 {
				return errors.Wrap(err, "unable to find a check for this system")
			}
			cb.logger.Info().Msg("no existing check found, creating")
			// attempt to create if not found and create flag ON
			b, err = cb.createCheckBundle()
			if err != nil {
				return errors.Wrap(err, "creating new check for this system")
			}
		}
		bundle = b
	}

	if bundle == nil {
		return errors.New("invalid Check object state, bundle is nil")
	}

	if bundle.Status != StatusActive {
		return &ErrNotActive{
			Err:      "not active",
			BundleID: bundle.CID,
			Checks:   strings.Join(bundle.Checks, ", "),
			Status:   bundle.Status,
		}
	}

	if viper.GetBool(config.KeyCheckUpdate) {
		b, err := cb.updateCheckBundle(bundle)
		if err != nil {
			return errors.Wrap(err, "updating check bundle")
		}
		bundle = b
	} else if viper.GetBool(config.KeyCheckUpdateMetricFilters) {
		b, err := cb.updateCheckBundleMetricFilters(bundle)
		if err != nil {
			return errors.Wrap(err, "updating check bundle metric filters")
		}
		bundle = b
	}

	cb.bundle = bundle

	return nil
}

func (cb *Bundle) fetchCheckBundle(cid string) (*apiclient.CheckBundle, error) {
	if cid == "" {
		return nil, errors.New("invalid cid (empty)")
	}

	if ok, _ := regexp.MatchString(`^[0-9]+$`, cid); ok {
		cid = "/check_bundle/" + cid
	}

	if ok, _ := regexp.MatchString(`^/check_bundle/[0-9]+$`, cid); !ok {
		return nil, errors.Errorf("invalid cid (%s)", cid)
	}

	bundle, err := cb.client.FetchCheckBundle(apiclient.CIDType(&cid))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to retrieve check bundle (%s)", cid)
	}

	if bundle.Status != StatusActive {
		return nil, &ErrNotActive{
			Err:      "not active",
			BundleID: bundle.CID,
			Checks:   strings.Join(bundle.Checks, ", "),
			Status:   bundle.Status,
		}
	}

	return bundle, nil
}

func (cb *Bundle) findCheckBundle() (*apiclient.CheckBundle, int, error) {
	target := viper.GetString(config.KeyCheckTarget)
	if target == "" {
		return nil, -1, errors.New("invalid check bundle target (empty)")
	}

	criteria := apiclient.SearchQueryType(fmt.Sprintf(`(active:1)(type:"json:nad")(target:"%s")`, target))
	bundles, err := cb.client.SearchCheckBundles(&criteria, nil)
	if err != nil {
		return nil, -1, errors.Wrap(err, "searching for check bundle")
	}

	found := len(*bundles)

	if found == 0 {
		return nil, found, errors.Errorf("no check bundles matched criteria (%s)", string(criteria))
	}

	if found > 1 {
		// if more than one bundle, find one created by the circonus-agent
		// if multiple bundles created by agent, error, otherwise return the one from the agent
		matched := 0
		idx := -1
		for i, b := range *bundles {
			if b.Notes != nil && strings.Contains(*b.Notes, release.NAME) {
				idx = i
				matched++
			}
		}
		if matched == 0 {
			cb.logger.Warn().Int("found", found).Int("matched", matched).Str("criteria", string(criteria)).Msgf("found multiple checks matching criteria, none created by %s", release.NAME)
			return nil, found, errors.Errorf("multiple checks (%d) found matching criteria (%s), none created by %s", found, string(criteria), release.NAME)
		}
		if matched == 1 {
			cb.logger.Warn().Int("found", found).Str("bundle", (*bundles)[idx].CID).Msgf("multiple checks found, using one created by %s", release.NAME)
			return &(*bundles)[idx], matched, nil
		}
		return nil, found, errors.Errorf("more than one (%d) check bundle matched criteria (%s)", found, string(criteria))
	}

	return &(*bundles)[0], found, nil
}

func (cb *Bundle) createCheckBundle() (*apiclient.CheckBundle, error) {

	// parse the first listen address to use as the required
	// URL in the check config
	var targetAddr string
	{
		serverList := viper.GetStringSlice(config.KeyListen)
		if len(serverList) == 0 {
			serverList = []string{defaults.Listen}
		}
		if serverList[0][0:1] == ":" {
			serverList[0] = "localhost" + serverList[0]
		}
		ta, err := config.ParseListen(serverList[0])
		if err != nil {
			cb.logger.Error().Err(err).Str("addr", serverList[0]).Msg("resolving address")
			return nil, errors.Wrap(err, "parsing listen address")
		}
		targetAddr = ta.String()
	}

	target := viper.GetString(config.KeyCheckTarget)
	if target == "" {
		return nil, errors.New("invalid check bundle target (empty)")
	}

	cfg := apiclient.NewCheckBundle()
	cfg.Target = target
	cfg.DisplayName = viper.GetString(config.KeyCheckTitle)
	if cfg.DisplayName == "" {
		cfg.DisplayName = cfg.Target + " /agent"
	}
	note := fmt.Sprintf("created by %s %s", release.NAME, release.VERSION)
	cfg.Notes = &note
	cfg.Type = "json:nad"
	cfg.Config = apiclient.CheckBundleConfig{apiconf.URL: "http://" + targetAddr + "/"}
	cfg.Metrics = []apiclient.CheckBundleMetric{}
	{
		period := viper.GetUint(config.KeyCheckPeriod)
		if period < 10 || period > 300 {
			period = defaults.CheckPeriod
		}
		cfg.Period = period
	}
	{
		timeout := viper.GetFloat64(config.KeyCheckTimeout)
		if timeout < 0 || timeout > 300 {
			timeout = defaults.CheckTimeout
		}
		cfg.Timeout = float32(timeout)
	}

	{ // get metric filter configuration
		filters, err := cb.getMetricFilters()
		if err != nil {
			return nil, errors.Wrap(err, "getting metric filters")
		}
		cfg.MetricFilters = filters
	}

	tags := viper.GetString(config.KeyCheckTags)
	if tags != "" {
		cfg.Tags = strings.Split(tags, ",")
	}

	brokerCID := viper.GetString(config.KeyCheckBroker)
	if brokerCID == "" || strings.ToLower(brokerCID) == "select" {
		brokerList, err := cb.client.FetchBrokers()
		if err != nil {
			return nil, errors.Wrap(err, "select broker")
		}

		broker, err := cb.selectBroker("json:nad", brokerList)
		if err != nil {
			return nil, errors.Wrap(err, "selecting broker to create check bundle")
		}

		brokerCID = broker.CID
	}

	if ok, _ := regexp.MatchString(`^[0-9]+$`, brokerCID); ok {
		brokerCID = "/broker/" + brokerCID
	}

	cfg.Brokers = []string{brokerCID}

	bundle, err := cb.client.CreateCheckBundle(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "creating check bundle")
	}

	return bundle, nil
}

func (cb *Bundle) updateCheckBundle(cfg *apiclient.CheckBundle) (*apiclient.CheckBundle, error) {

	// parse the first listen address to use as the required
	// URL in the check config
	var targetAddr string
	{
		serverList := viper.GetStringSlice(config.KeyListen)
		if len(serverList) == 0 {
			serverList = []string{defaults.Listen}
		}
		if serverList[0][0:1] == ":" {
			serverList[0] = "localhost" + serverList[0]
		}
		ta, err := config.ParseListen(serverList[0])
		if err != nil {
			cb.logger.Error().Err(err).Str("addr", serverList[0]).Msg("resolving address")
			return nil, errors.Wrap(err, "parsing listen address")
		}
		targetAddr = ta.String()
	}

	target := viper.GetString(config.KeyCheckTarget)
	if target == "" {
		return nil, errors.New("invalid check bundle target (empty)")
	}

	cfg.Target = target
	cfg.DisplayName = viper.GetString(config.KeyCheckTitle)
	if cfg.DisplayName == "" {
		cfg.DisplayName = cfg.Target + " /agent"
	}
	note := fmt.Sprintf("updated by %s %s", release.NAME, release.VERSION)
	cfg.Notes = &note
	cfg.Config = apiclient.CheckBundleConfig{apiconf.URL: "http://" + targetAddr + "/"}
	cfg.Metrics = []apiclient.CheckBundleMetric{}
	{
		period := viper.GetUint(config.KeyCheckPeriod)
		if period < 10 || period > 300 {
			period = defaults.CheckPeriod
		}
		cfg.Period = period
	}
	{
		timeout := viper.GetFloat64(config.KeyCheckTimeout)
		if timeout < 0 || timeout > 300 {
			timeout = defaults.CheckTimeout
		}
		cfg.Timeout = float32(timeout)
	}

	{ // get metric filter configuration
		filters, err := cb.getMetricFilters()
		if err != nil {
			return nil, errors.Wrap(err, "getting metric filters")
		}
		cfg.MetricFilters = filters
	}

	tags := viper.GetString(config.KeyCheckTags)
	if tags != "" {
		cfg.Tags = strings.Split(tags, ",")
	}

	brokerCID := viper.GetString(config.KeyCheckBroker)
	if brokerCID != "" && brokerCID != "select" {
		if ok, _ := regexp.MatchString(`^[0-9]+$`, brokerCID); ok {
			brokerCID = "/broker/" + brokerCID
		}
		cfg.Brokers = []string{brokerCID}
	}

	bundle, err := cb.client.UpdateCheckBundle(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "updating check bundle")
	}

	return bundle, nil
}

type MetricFilterFile struct {
	Filters [][]string `json:"metric_filters"`
}

func (cb *Bundle) getMetricFilters() ([][]string, error) {
	mff := viper.GetString(config.KeyCheckMetricFilterFile)
	if mff != "" {
		data, err := ioutil.ReadFile(mff)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, errors.Wrapf(err, "reading %s", mff)
			}
		} else {
			var filters MetricFilterFile
			if err := json.Unmarshal(data, &filters); err != nil {
				return nil, errors.Wrap(err, "parsing metric filters")
			}
			cb.logger.Debug().Interface("filters", filters).Str("file", mff).Msg("using metric filter file")
			return filters.Filters, nil
		}
	}

	if viper.GetString(config.KeyCheckMetricFilters) != "" {
		var filters [][]string
		if err := json.Unmarshal([]byte(viper.GetString(config.KeyCheckMetricFilters)), &filters); err != nil {
			return nil, errors.Wrap(err, "parsing check bundle metric filters")
		}
		cb.logger.Debug().Interface("filters", filters).Msg("using metric filters option")
		return filters, nil
	}

	cb.logger.Debug().Interface("filters", defaults.CheckMetricFilters).Msg("using default metric filters")
	return defaults.CheckMetricFilters, nil
}

func (cb *Bundle) updateCheckBundleMetricFilters(cfg *apiclient.CheckBundle) (*apiclient.CheckBundle, error) {
	filters, err := cb.getMetricFilters()
	if err != nil {
		return nil, errors.Wrap(err, "getting metric filters")
	}
	cfg.MetricFilters = filters
	cb.logger.Info().Interface("filters", filters).Msg("updating check bundle metric filters")
	bundle, err := cb.client.UpdateCheckBundle(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "updating metric filters")
	}
	return bundle, nil
}
