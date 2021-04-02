// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package bundle

import (
	"fmt"
	"math/rand"
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/go-apiclient"
	"github.com/spf13/viper"
)

// Select a broker for use when creating a check, if a specific broker
// was not specified.
func (cb *Bundle) selectBroker(checkType string, brokerList *[]apiclient.Broker) (*apiclient.Broker, error) {
	if checkType == "" {
		return nil, errInvalidCheckType
	}

	if brokerList == nil {
		return nil, errInvalidBrokerList
	}

	if len(*brokerList) == 0 {
		return nil, errInvalidBrokerListEmpty
	}

	validBrokers := make(map[string]apiclient.Broker)
	haveEnterprise := false
	threshold := 10 * time.Second

	for _, broker := range *brokerList {
		broker := broker
		dur, ok := cb.isValidBroker(&broker, checkType)
		if !ok {
			continue
		}

		switch {
		case dur > threshold:
			continue
		case dur == threshold:
			validBrokers[broker.CID] = broker
		case dur < threshold:
			if len(validBrokers) > 0 {
				// we want the fastest broker(s), reset list if any
				// slower brokers were already added
				validBrokers = make(map[string]apiclient.Broker)
			}
			haveEnterprise = false
			threshold = dur
			validBrokers[broker.CID] = broker
		}

		if broker.Type == "enterprise" {
			haveEnterprise = true
		}
	}

	if !haveEnterprise && viper.GetBool(config.KeyMultiAgent) {
		return nil, errIvalidEnterpriseForMultiAgent
	}

	if haveEnterprise { // eliminate non-enterprise brokers from valid brokers
		for k, v := range validBrokers {
			if v.Type != "enterprise" {
				delete(validBrokers, k)
			}
		}
	}

	if len(validBrokers) == 0 {
		return nil, fmt.Errorf("%d broker(s): %w", len(*brokerList), errNoValidBrokersFound)
	}

	var selectedBroker apiclient.Broker
	validBrokerKeys := reflect.ValueOf(validBrokers).MapKeys()
	if len(validBrokerKeys) == 1 {
		selectedBroker = validBrokers[validBrokerKeys[0].String()]
	} else {
		selectedBroker = validBrokers[validBrokerKeys[rand.Intn(len(validBrokerKeys))].String()] //nolint:gosec
	}

	cb.logger.Debug().Str("broker", selectedBroker.Name).Msg("selected")

	return &selectedBroker, nil
}

// Is the broker valid (active, supports check type, and reachable).
func (cb *Bundle) isValidBroker(broker *apiclient.Broker, checkType string) (time.Duration, bool) {
	if broker == nil {
		return 0, false
	}
	if checkType == "" {
		return 0, false
	}

	var brokerHost string
	var brokerPort string
	var connDuration time.Duration
	valid := false

	for _, detail := range broker.Details {
		detail := detail

		// broker must be active
		if detail.Status != cb.statusActiveBroker {
			cb.logger.Debug().
				Str("broker", broker.Name).
				Str("instance", detail.CN).
				Msg("not active, skipping")
			continue
		}

		// broker must have module loaded for the check type to be used
		if !brokerSupportsCheckType(checkType, &detail) {
			cb.logger.Debug().
				Str("broker", broker.Name).
				Str("instance", detail.CN).
				Str("type", checkType).
				Msg("unsupported check type, skipping")
			continue
		}
		// broker must have httptrap for multi-agent to function correctly
		if viper.GetBool(config.KeyMultiAgent) {
			if !brokerSupportsCheckType("httptrap", &detail) {
				cb.logger.Debug().
					Str("broker", broker.Name).
					Str("instance", detail.CN).
					Str("type", "httptrap").
					Msg("unsupported check type, skipping - required for multi-agent")
				continue
			}
		}

		if detail.ExternalPort != 0 {
			brokerPort = strconv.Itoa(int(detail.ExternalPort))
		} else {
			if detail.Port != nil && *detail.Port != 0 {
				brokerPort = strconv.Itoa(int(*detail.Port))
			} else {
				brokerPort = "43191"
			}
		}

		if detail.ExternalHost != nil && *detail.ExternalHost != "" {
			brokerHost = *detail.ExternalHost
		} else {
			if detail.IP == nil || *detail.IP == "" {
				cb.logger.Debug().
					Str("broker", broker.Name).
					Str("instance", detail.CN).
					Msg("no external host or ip, skipping")
				continue
			}
			brokerHost = *detail.IP
		}

		if brokerHost == "trap.noit.circonus.net" && brokerPort != "443" {
			brokerPort = "443"
		}

		minDelay := int(200 * time.Millisecond)
		maxDelay := int(2 * time.Second)

		for attempt := 1; attempt <= cb.brokerMaxRetries; attempt++ {
			start := time.Now()
			// broker must be reachable and respond within designated time
			conn, err := net.DialTimeout("tcp", net.JoinHostPort(brokerHost, brokerPort), cb.brokerMaxResponseTime)
			if err == nil {
				connDuration = time.Since(start)
				conn.Close()
				valid = true
				break
			}

			delay := time.Duration(rand.Intn(maxDelay-minDelay) + minDelay) //nolint:gosec

			cb.logger.Warn().
				Err(err).
				Str("delay", delay.String()).
				Str("broker", broker.Name).
				Str("instance", detail.CN).
				Int("attempt", attempt).
				Int("retries", cb.brokerMaxRetries).
				Msg("unable to connect, retrying")

			time.Sleep(delay)
		}

		if valid {
			cb.logger.Debug().
				Str("broker", broker.Name).
				Str("instance", detail.CN).
				Msg("valid")
			break
		}
	}

	return connDuration, valid
}

// brokerSupportsCheckType verifies a broker supports the check type to be used.
func brokerSupportsCheckType(checkType string, details *apiclient.BrokerDetail) bool {
	if checkType == "" {
		return false
	}
	if details == nil {
		return false
	}

	baseType := checkType

	if idx := strings.Index(baseType, ":"); idx > 0 {
		baseType = baseType[0:idx]
	}

	for _, module := range details.Modules {
		if module == baseType {
			return true
		}
	}

	return false
}
