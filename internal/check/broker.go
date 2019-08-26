// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"net"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/go-apiclient"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// brokerTLSConfig returns the correct TLS configuration for the broker
func (c *Check) brokerTLSConfig(cid string, reverseURL *url.URL) (*tls.Config, string, error) {
	if cid == "" {
		return nil, "", errors.New("invalid broker cid (empty)")
	}

	bcid := cid

	if ok, _ := regexp.MatchString(`^[0-9]+$`, bcid); ok {
		bcid = "/broker/" + cid
	}

	if ok, _ := regexp.MatchString(`^/broker/[0-9]+$`, bcid); !ok {
		return nil, "", errors.Errorf("invalid broker cid (%s)", cid)
	}

	broker, err := c.client.FetchBroker(apiclient.CIDType(&bcid))
	if err != nil {
		return nil, "", errors.Wrapf(err, "unable to retrieve broker (%s)", cid)
	}

	cn, err := c.getBrokerCN(broker, reverseURL)
	if err != nil {
		return nil, "", err
	}
	cert, err := c.fetchBrokerCA()
	if err != nil {
		return nil, "", err
	}
	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM(cert) {
		return nil, "", errors.New("unable to add Broker CA Certificate to x509 cert pool")
	}

	tlsConfig := &tls.Config{
		RootCAs:    cp,
		ServerName: cn,
	}

	c.logger.Debug().Str("CN", cn).Msg("setting tls CN")

	return tlsConfig, cn, nil
}

func (c *Check) getBrokerCN(broker *apiclient.Broker, reverseURL *url.URL) (string, error) {
	host := reverseURL.Hostname()

	// OK...
	//
	// mtev_reverse can have an IP or an FQDN for the host portion
	// it used to be that when it was an IP, the CN was needed in order to verify TLS connections
	// otherwise, the FQDN was valid. now, the FQDN may be valid for the cert or it may not be...

	cn := ""

	for _, detail := range broker.Details {
		// certs are generated against the CN (in theory)
		// 1. find the right broker instance with matching IP or external hostname
		// 2. set the tls.Config.ServerName to whatever that instance's CN is currently
		// 3. cert will be valid for TLS conns (in theory)
		if detail.IP != nil && *detail.IP == host {
			cn = detail.CN
			break
		}
		if detail.ExternalHost != nil && *detail.ExternalHost == host {
			cn = detail.CN
			break
		}
	}

	if cn == "" {
		return "", errors.Errorf("unable to match reverse URL host (%s) to broker", host)
	}

	return cn, nil
}

func (c *Check) fetchBrokerCA() ([]byte, error) {
	// use local file if specified
	file := viper.GetString(config.KeyReverseBrokerCAFile)
	if file != "" {
		cert, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, errors.Wrapf(err, "reading specified broker-ca-file (%s)", file)
		}
		return cert, nil
	}

	// otherwise, try the api
	data, err := c.client.Get("/pki/ca.crt")
	if err != nil {
		return nil, errors.Wrap(err, "fetching Broker CA certificate")
	}

	type cacert struct {
		Contents string `json:"contents"`
	}

	var cadata cacert

	if err := json.Unmarshal(data, &cadata); err != nil {
		return nil, errors.Wrap(err, "parsing Broker CA certificate")
	}

	if cadata.Contents == "" {
		return nil, errors.Errorf("no Broker CA certificate in response (%#v)", string(data))
	}

	return []byte(cadata.Contents), nil
}

// Select a broker for use when creating a check, if a specific broker
// was not specified.
func (c *Check) selectBroker(checkType string) (*apiclient.Broker, error) {
	brokerList, err := c.client.FetchBrokers()
	if err != nil {
		return nil, errors.Wrap(err, "select broker")
	}

	if len(*brokerList) == 0 {
		return nil, errors.New("no brokers returned from API")
	}

	validBrokers := make(map[string]apiclient.Broker)
	haveEnterprise := false
	threshold := 10 * time.Second

	for _, broker := range *brokerList {
		broker := broker
		dur, ok := c.isValidBroker(&broker, checkType)
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

	if haveEnterprise { // eliminate non-enterprise brokers from valid brokers
		for k, v := range validBrokers {
			if v.Type != "enterprise" {
				delete(validBrokers, k)
			}
		}
	}

	if len(validBrokers) == 0 {
		return nil, errors.Errorf("found %d broker(s), zero are valid", len(*brokerList))
	}

	var selectedBroker apiclient.Broker
	validBrokerKeys := reflect.ValueOf(validBrokers).MapKeys()
	if len(validBrokerKeys) == 1 {
		selectedBroker = validBrokers[validBrokerKeys[0].String()]
	} else {
		selectedBroker = validBrokers[validBrokerKeys[rand.Intn(len(validBrokerKeys))].String()]
	}

	c.logger.Debug().Str("broker", selectedBroker.Name).Msg("selected")

	return &selectedBroker, nil
}

// Is the broker valid (active, supports check type, and reachable)
func (c *Check) isValidBroker(broker *apiclient.Broker, checkType string) (time.Duration, bool) {
	var brokerHost string
	var brokerPort string
	var connDuration time.Duration
	valid := false

	for _, detail := range broker.Details {
		detail := detail

		// broker must be active
		if detail.Status != c.statusActiveBroker {
			c.logger.Debug().Str("broker", broker.Name).Msg("not active, skipping")
			continue
		}

		// broker must have module loaded for the check type to be used
		if !brokerSupportsCheckType(checkType, &detail) {
			c.logger.Debug().Str("broker", broker.Name).Str("type", checkType).Msg("unsupported check type, skipping")
			continue
		}

		if detail.ExternalPort != 0 {
			brokerPort = strconv.Itoa(int(detail.ExternalPort))
		} else {
			if *detail.Port != 0 {
				brokerPort = strconv.Itoa(int(*detail.Port))
			} else {
				brokerPort = "43191"
			}
		}

		if detail.ExternalHost != nil && *detail.ExternalHost != "" {
			brokerHost = *detail.ExternalHost
		} else {
			brokerHost = *detail.IP
		}

		if brokerHost == "trap.noit.circonus.net" && brokerPort != "443" {
			brokerPort = "443"
		}

		minDelay := int(200 * time.Millisecond)
		maxDelay := int(2 * time.Second)

		for attempt := 1; attempt <= c.brokerMaxRetries; attempt++ {
			start := time.Now()
			// broker must be reachable and respond within designated time
			conn, err := net.DialTimeout("tcp", net.JoinHostPort(brokerHost, brokerPort), c.brokerMaxResponseTime)
			if err == nil {
				connDuration = time.Since(start)
				conn.Close()
				valid = true
				break
			}

			delay := time.Duration(rand.Intn(maxDelay-minDelay) + minDelay)

			c.logger.Warn().
				Err(err).
				Str("delay", delay.String()).
				Str("broker", broker.Name).
				Int("attempt", attempt).
				Int("retries", c.brokerMaxRetries).
				Msg("unable to connect, retrying")

			time.Sleep(delay)
		}

		if valid {
			c.logger.Debug().Str("broker", broker.Name).Msg("valid")
			break
		}
	}

	return connDuration, valid
}

// brokerSupportsCheckType verifies a broker supports the check type to be used
func brokerSupportsCheckType(checkType string, details *apiclient.BrokerDetail) bool {
	baseType := string(checkType)

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
