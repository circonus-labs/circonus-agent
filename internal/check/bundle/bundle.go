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
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-agent/internal/release"
	"github.com/circonus-labs/go-apiclient"
	apiconf "github.com/circonus-labs/go-apiclient/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// Bundle exposes the check bundle management interface.
type Bundle struct {
	statusActiveBroker    string
	bundle                *apiclient.CheckBundle
	client                API
	logger                zerolog.Logger
	brokerMaxResponseTime time.Duration
	brokerMaxRetries      int
	sync.Mutex
}

var (
	// broker errors.

	errInvalidCheckType              = fmt.Errorf("invalid check type (empty)")
	errInvalidBrokerList             = fmt.Errorf("invalid broker list (nil)")
	errInvalidBrokerListEmpty        = fmt.Errorf("invalid broker list (empty)")
	errIvalidEnterpriseForMultiAgent = fmt.Errorf("no enterprise brokers satisfy multi-agent requirements")
	errNoValidBrokersFound           = fmt.Errorf("no valid brokers found")

	// bundle errors.

	ErrUninitialized           = fmt.Errorf("uninitialized check bundle")
	errInvalidCheckBundle      = fmt.Errorf("invalid check bundle (nil)")
	errInvalidReverseRules     = fmt.Errorf("invalid check bundle (0 reverse urls)")
	errNoChecksInBundle        = fmt.Errorf("no checks found in check bundle")
	errInvalidCheckIndex       = fmt.Errorf("invalid check index")
	errInvalidCheckBundleState = fmt.Errorf("invalid Check object state, bundle is nil")
	errInvalidCID              = fmt.Errorf("invalid CID")
	errInvalidTarget           = fmt.Errorf("invalid check bundle target (empty)")
	errNoBundlesMatched        = fmt.Errorf("no check bundles matched")

	// other errors.

	errInvalidClient = fmt.Errorf("invalid client (nil)")
)

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
		return nil, errInvalidClient
	}

	cb := Bundle{
		client:                client,
		brokerMaxResponseTime: 500 * time.Millisecond,
		brokerMaxRetries:      5,
		bundle:                nil,
		logger:                log.With().Str("pkg", "bundle").Logger(),
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
		cb.logger.Info().Msg("check management disabled")
		return &cb, nil // if we don't need a check, return a NOP object
	}

	// initialize the check bundle
	if err := cb.initCheckBundle(cid, isCreate); err != nil {
		return nil, fmt.Errorf("initializing check bundle: %w", err)
	}

	// ensure a) the global check bundle id is set and b) it is set correctly
	// to the check bundle actually being used - need to do this even if the
	// check was created initially since user 'nobody' cannot create or update
	// the configuration (if one was used)
	viper.Set(config.KeyCheckBundleID, cb.bundle.CID)
	cb.logger.Debug().Interface("config", cb.bundle).Msg("using check bundle config")

	return &cb, nil
}

// CID returns the check bundle cid.
func (cb *Bundle) CID() (string, error) {
	cb.Lock()
	defer cb.Unlock()

	if cb.bundle != nil {
		return cb.bundle.CID, nil
	}

	return "", ErrUninitialized
}

// Period returns check bundle period (intetrval between when broker should make requests).
func (cb *Bundle) Period() (uint, error) {
	cb.Lock()
	defer cb.Unlock()

	if cb.bundle != nil {
		return cb.bundle.Period, nil
	}

	return 0, ErrUninitialized
}

// SubmissionURL returns the submission url (derived from mtev_reverse).
func (cb *Bundle) SubmissionURL() (string, error) {
	if cb.bundle == nil {
		return "", errInvalidCheckBundle
	}
	if len(cb.bundle.ReverseConnectURLs) == 0 {
		return "", errInvalidReverseRules
	}

	// submission url from mtev_reverse url, given:
	//
	// mtev_reverse://FQDN_OR_IP:PORT/check/UUID
	// config.reverse:secret_key "sec_string"
	//
	// use: https://FQDN_OR_IP:PORT/module/httptrap/UUID/sec_string
	//
	mtevReverse := cb.bundle.ReverseConnectURLs[0]
	mtevSecret := cb.bundle.Config[apiconf.ReverseSecretKey]
	submissionURL := strings.Replace(strings.Replace(mtevReverse, "mtev_reverse", "https", 1), "check", "module/httptrap", 1)
	submissionURL += "/" + mtevSecret
	return submissionURL, nil
}

// Refresh re-loads the check bundle using the API (sets metric states if check bundle is managed).
func (cb *Bundle) Refresh() error {
	cb.Lock()
	defer cb.Unlock()

	if cb.bundle == nil {
		return ErrUninitialized
	}

	cb.logger.Debug().Msg("refreshing check configuration using API")

	b, err := cb.fetchCheckBundle(viper.GetString(config.KeyCheckBundleID))
	if err != nil {
		return fmt.Errorf("refresh check, fetching check: %w", err)
	}

	cb.bundle = b

	return nil
}

// CheckCID returns the check cid at the passed index within the check bundle's
// checks array or an error if bundle not initialized.
func (cb *Bundle) CheckCID(idx uint) (string, error) {
	cb.Lock()
	defer cb.Unlock()

	if cb.bundle == nil {
		return "", ErrUninitialized
	}
	if len(cb.bundle.Checks) == 0 {
		return "", errNoChecksInBundle
	}
	if int(idx) > len(cb.bundle.Checks) {
		return "", fmt.Errorf("index(%d) checks(%d): %w", idx, len(cb.bundle.Checks), errInvalidCheckIndex)
	}

	return cb.bundle.Checks[idx], nil
}

// initCheck initializes the check for the agent.
// 1. load a saved check bundle
// 2. fetch a check explicitly provided via CID
// 3. search for a check matching the current system
// 4. create a check for the system if --check-create specified
// if fetched, found, or created - set Check.bundle
// otherwise, return an error.
func (cb *Bundle) initCheckBundle(cid string, create bool) error {
	var bundle *apiclient.CheckBundle

	{
		// first, try to load a previously saved bundle
		// NOTE: takes precedence over configured check bundle cid
		b, loaded := cb.loadBundle()
		if loaded {
			// set cid to loaded cid so bundle can be
			// refreshed and verified to be active
			cid = b.CID
		}
	}

	// if explicit cid loaded or configured, attempt to fetch check bundle using cid
	if cid != "" {
		b, err := cb.fetchCheckBundle(cid)
		if err != nil {
			return fmt.Errorf("fetching check for cid %s: %w", cid, err)
		}
		bundle = b
	} else {
		// if no cid configured, attempt to find check bundle matching this system
		b, found, err := cb.findCheckBundle()
		if err != nil {
			if !create || found != 0 {
				return fmt.Errorf("unable to find a check for this system: %w", err)
			}
			cb.logger.Info().Msg("no existing check found, creating")
			// attempt to create if not found and create flag ON
			b, err = cb.createCheckBundle()
			if err != nil {
				return fmt.Errorf("creating new check for this system: %w", err)
			}
		}
		bundle = b
	}

	if bundle == nil {
		return errInvalidCheckBundleState
	}

	if bundle.Status != StatusActive {
		return &ErrNotActive{
			Err:      "not active",
			BundleID: bundle.CID,
			Checks:   strings.Join(bundle.Checks, ", "),
			Status:   bundle.Status,
		}
	}

	switch {
	case viper.GetBool(config.KeyCheckUpdate):
		cb.logger.Info().Str("cid", bundle.CID).Msg("updating check bundle")
		b, err := cb.updateCheckBundle(bundle)
		if err != nil {
			return fmt.Errorf("updating check bundle: %w", err)
		}
		bundle = b
	case viper.GetBool(config.KeyCheckUpdateMetricFilters):
		cb.logger.Info().Str("cid", bundle.CID).Msg("updating check bundle metric filters and host tags")
		b, err := cb.updateCheckBundleMetricFilters(bundle)
		if err != nil {
			return fmt.Errorf("updating check bundle metric filters: %w", err)
		}
		bundle = b
	default:
		cb.logger.Info().Str("cid", bundle.CID).Msg("updating check bundle host tags")
		b, err := cb.updateCheckBundleTags(bundle)
		if err != nil {
			return fmt.Errorf("updating check bundle tags: %w", err)
		}
		bundle = b
	}

	cb.bundle = bundle
	cb.logger.Info().Str("cid", cb.bundle.CID).Str("name", cb.bundle.DisplayName).Msg("using check bundle")

	// try to save the bundle to enable --check-delete
	// this will save a created check, an updated check, a found check, etc.
	cb.saveBundle(bundle)

	return nil
}

func (cb *Bundle) fetchCheckBundle(cid string) (*apiclient.CheckBundle, error) {
	if cid == "" {
		return nil, fmt.Errorf("empty: %w", errInvalidCID)
	}

	if ok, _ := regexp.MatchString(`^[0-9]+$`, cid); ok {
		cid = "/check_bundle/" + cid
	}

	if ok, _ := regexp.MatchString(`^/check_bundle/[0-9]+$`, cid); !ok {
		return nil, fmt.Errorf("cid %s: %w", cid, errInvalidCID)
	}

	bundle, err := cb.client.FetchCheckBundle(apiclient.CIDType(&cid))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve check bundle (%s): %w", cid, err)
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
		return nil, -1, errInvalidTarget
	}

	criteria := apiclient.SearchQueryType(fmt.Sprintf(`(active:1)(type:"`+defaults.CheckType+`")(target:"%s")`, target))
	bundles, err := cb.client.SearchCheckBundles(&criteria, nil)
	if err != nil {
		return nil, -1, fmt.Errorf("searching for check bundle: %w", err)
	}

	found := len(*bundles)

	if found == 0 {
		return nil, found, fmt.Errorf("criteria - %s: %w", string(criteria), errNoBundlesMatched)
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
			cb.logger.Warn().
				Int("found", found).
				Int("matched", matched).
				Str("criteria", string(criteria)).
				Msgf("multiple check bundles found, none created by (%s)", release.NAME)
			return nil, matched, fmt.Errorf("multiple check bundles (%d) found matching criteria (%s), none created by (%s)", found, string(criteria), release.NAME) //nolint:goerr113
		}
		if matched == 1 {
			cb.logger.Warn().
				Int("found", found).
				Int("matched", matched).
				Str("criteria", string(criteria)).
				Str("bundle", (*bundles)[idx].CID).
				Msgf("multiple check bundles found, using one created by (%s)", release.NAME)
			return &(*bundles)[idx], matched, nil
		}
		return nil, found, fmt.Errorf("multiple check bundles (%d) found matching criteria (%s) created by (%s)", matched, string(criteria), release.NAME) //nolint:goerr113
	}

	// search does not always return a valid mtev_reverse url (e.g. mtev_reverse://:43191/ no host/ip)
	// re-fetch the bundle by its cid does however fill out correctly.

	cid := (*bundles)[0].CID
	// cb.logger.Debug().Str("cid", cid).Msg("fetching check bundle found by search")
	b, err := cb.fetchCheckBundle(cid)
	if err != nil {
		return nil, 0, fmt.Errorf("unable to retrieve check bundle by id (%s): %w", cid, err)
	}
	// cb.logger.Debug().Interface("search", *bundles).Interface("fetch", b).Msg("bundles")

	return b, found, nil
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
		if serverList[0][0:1] == ":" && viper.GetBool(config.KeyReverse) {
			serverList[0] = "localhost" + serverList[0]
		}
		ta, err := config.ParseListen(serverList[0])
		if err != nil {
			cb.logger.Error().Err(err).Str("addr", serverList[0]).Msg("resolving address")
			return nil, fmt.Errorf("parsing listen address: %w", err)
		}
		targetAddr = ta.String()
	}

	target := viper.GetString(config.KeyCheckTarget)
	if target == "" {
		return nil, errInvalidTarget
	}

	if targetAddr[0:1] == ":" && !viper.GetBool(config.KeyReverse) {
		targetAddr = target + targetAddr
	}

	cfg := apiclient.NewCheckBundle()
	cfg.Target = target
	cfg.DisplayName = viper.GetString(config.KeyCheckTitle)
	if cfg.DisplayName == "" {
		cfg.DisplayName = cfg.Target + " /agent"
	}
	note := fmt.Sprintf("created by %s %s", release.NAME, release.VERSION)
	cfg.Notes = &note
	cfg.Tags = cb.getHostTags()
	cfg.Type = defaults.CheckType
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
			return nil, fmt.Errorf("getting metric filters: %w", err)
		}
		cfg.MetricFilters = filters
	}

	brokerCID := viper.GetString(config.KeyCheckBroker)
	if brokerCID == "" || strings.ToLower(brokerCID) == "select" {
		brokerList, err := cb.client.FetchBrokers()
		if err != nil {
			return nil, fmt.Errorf("select broker: %w", err)
		}

		broker, err := cb.selectBroker(defaults.CheckType, brokerList)
		if err != nil {
			return nil, fmt.Errorf("selecting broker to create check bundle: %w", err)
		}

		brokerCID = broker.CID
	}

	if ok, _ := regexp.MatchString(`^[0-9]+$`, brokerCID); ok {
		brokerCID = "/broker/" + brokerCID
	}

	cfg.Brokers = []string{brokerCID}

	bundle, err := cb.client.CreateCheckBundle(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating check bundle: %w", err)
	}

	return bundle, nil
}

// updateCheckBundle will update all check bundle settings/tags/filters controlled by agent.
func (cb *Bundle) updateCheckBundle(cfg *apiclient.CheckBundle) (*apiclient.CheckBundle, error) {

	// this is an explicit update - all configurable values will be overwritten with their configuration settings

	// parse the first listen address to use as the required
	// URL in the check config
	var targetAddr string
	{
		serverList := viper.GetStringSlice(config.KeyListen)
		if len(serverList) == 0 {
			serverList = []string{defaults.Listen}
		}
		if serverList[0][0:1] == ":" && viper.GetBool(config.KeyReverse) {
			serverList[0] = "localhost" + serverList[0]
		}
		ta, err := config.ParseListen(serverList[0])
		if err != nil {
			cb.logger.Error().Err(err).Str("addr", serverList[0]).Msg("resolving address")
			return nil, fmt.Errorf("parsing listen address: %w", err)
		}
		targetAddr = ta.String()
	}

	target := viper.GetString(config.KeyCheckTarget)
	if target == "" {
		return nil, errInvalidTarget
	}

	if targetAddr[0:1] == ":" && !viper.GetBool(config.KeyReverse) {
		targetAddr = target + targetAddr
	}

	cfg.Target = target
	cfg.DisplayName = viper.GetString(config.KeyCheckTitle)
	if cfg.DisplayName == "" {
		cfg.DisplayName = cfg.Target + " /agent"
	}
	note := fmt.Sprintf("updated by %s %s", release.NAME, release.VERSION)
	cfg.Notes = &note
	cfg.Tags = cb.getHostTags()
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
			return nil, fmt.Errorf("getting metric filters: %w", err)
		}
		cfg.MetricFilters = filters
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
		return nil, fmt.Errorf("updating check bundle: %w", err)
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
				return nil, fmt.Errorf("reading %s: %w", mff, err)
			}
		} else {
			var filters MetricFilterFile
			if err := json.Unmarshal(data, &filters); err != nil {
				return nil, fmt.Errorf("json parse - metric filters file: %w", err)
			}
			cb.logger.Debug().Interface("filters", filters).Str("file", mff).Msg("using metric filter file")
			return filters.Filters, nil
		}
	}

	if viper.GetString(config.KeyCheckMetricFilters) != "" {
		var filters [][]string
		if err := json.Unmarshal([]byte(viper.GetString(config.KeyCheckMetricFilters)), &filters); err != nil {
			return nil, fmt.Errorf("json parse - metric filters cfg: %w", err)
		}
		cb.logger.Debug().Interface("filters", filters).Msg("using metric filters option")
		return filters, nil
	}

	cb.logger.Debug().Interface("filters", defaults.CheckMetricFilters).Msg("using default metric filters")
	return defaults.CheckMetricFilters, nil
}

// updateCheckBundleMetricFilters only (forced); then merge/update tags.
func (cb *Bundle) updateCheckBundleMetricFilters(cfg *apiclient.CheckBundle) (*apiclient.CheckBundle, error) {

	// this is an explicit, forced update and bundle metric filters will be overwritten with configured filters

	filters, err := cb.getMetricFilters()
	if err != nil {
		return nil, fmt.Errorf("getting metric filters: %w", err)
	}
	cfg.MetricFilters = filters

	cb.logger.Info().Interface("filters", filters).Msg("updating check bundle metric filters")
	bundle, err := cb.client.UpdateCheckBundle(cfg)
	if err != nil {
		return nil, fmt.Errorf("updating metric filters: %w", err)
	}

	return cb.updateCheckBundleTags(bundle)
}

// updateCheckBundleTags only, merge w/user-added; update any host tags where value has changed.
func (cb *Bundle) updateCheckBundleTags(cfg *apiclient.CheckBundle) (*apiclient.CheckBundle, error) {

	// this is a passive update, so tags are merged with user tags and any host tags are updated to new values if they've changed

	updateCheck := false

	newTags := cb.getHostTags()

	if len(cfg.Tags) == 0 {
		updateCheck = true
		cfg.Tags = newTags
	} else {
		updTags := make([]string, len(cfg.Tags))
		copy(updTags, cfg.Tags)

		for _, tag := range newTags {
			parts := strings.SplitN(tag, ":", 2)
			tagCat := parts[0] + ":"
			found := false
			replace := false
			repIdx := 0
			for i, et := range updTags {
				if strings.HasPrefix(et, tagCat) {
					found = true
					if et != tag {
						replace = true
						repIdx = i
					}
					break
				}
			}
			if found {
				if replace {
					updateCheck = true
					updTags[repIdx] = tag
				}
			} else {
				updateCheck = true
				updTags = append(updTags, tag) //nolint:makezero
			}
		}

		if updateCheck {
			cfg.Tags = updTags
		}
	}

	if updateCheck {
		bundle, err := cb.client.UpdateCheckBundle(cfg)
		if err != nil {
			return nil, fmt.Errorf("updating check bundle: %w", err)
		}
		return bundle, nil
	}

	return cfg, nil
}

// saveBundle stores the bundle in a json file in etc/check_bundle.json if it is writeable
// the presence of this file allows --check-delete to work.
func (cb *Bundle) saveBundle(bundle *apiclient.CheckBundle) {
	data, err := json.Marshal(bundle)
	if err != nil {
		cb.logger.Error().Err(err).Msg("marshaling check bundle")
		return
	}
	if err := os.WriteFile(defaults.CheckBundleFile, data, 0600); err != nil {
		cb.logger.Error().Err(err).Msg("saving check bundle, --check-delete is disabled")
	}
}

// loadBundle reads a previously saved check bundle and uses it.
func (cb *Bundle) loadBundle() (*apiclient.CheckBundle, bool) {
	bundleFile := defaults.CheckBundleFile
	var bundle apiclient.CheckBundle
	data, err := os.ReadFile(bundleFile)
	if err != nil {
		if !os.IsNotExist(err) {
			cb.logger.Warn().Err(err).Str("bundle_file", bundleFile).Msg("loading check bundle")
		}
		return nil, false
	}

	if err := json.Unmarshal(data, &bundle); err != nil {
		cb.logger.Warn().Err(err).Str("bundle_file", bundleFile).Msg("parsing check bundle")
		return nil, false
	}

	return &bundle, true
}
