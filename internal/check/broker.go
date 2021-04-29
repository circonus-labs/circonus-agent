// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/spf13/viper"
)

var (
	errBrokerNotInitialized  = fmt.Errorf("broker not initialized")
	errBrokerAddCACertToPool = fmt.Errorf("unable to add Broker CA Certificate to x509 cert pool")
	errBrokerMatchRevURLHost = fmt.Errorf("unable to match reverse URL host to broker")
)

// brokerTLSConfig returns the correct TLS configuration for the broker.
func (c *Check) brokerTLSConfig(reverseURL *url.URL) (*tls.Config, string, error) {
	if c.broker == nil {
		return nil, "", errBrokerNotInitialized
	}

	cn, err := c.getBrokerCN(reverseURL)
	if err != nil {
		return nil, "", err
	}
	cert, err := c.fetchBrokerCA()
	if err != nil {
		return nil, "", err
	}
	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM(cert) {
		return nil, "", errBrokerAddCACertToPool
	}

	tlsConfig := &tls.Config{
		// RootCAs:    cp, // go1.15 see VerifyConnection below - until CN added to SAN in broker certs
		ServerName: cn,
		MinVersion: tls.VersionTLS12,
		// NOTE: This does NOT disable VerifyConnection()
		InsecureSkipVerify: true, //nolint:gosec
		VerifyConnection: func(cs tls.ConnectionState) error {
			commonName := cs.PeerCertificates[0].Subject.CommonName
			if commonName != cs.ServerName {
				return fmt.Errorf("invalid certificate name %q, expected %q", commonName, cs.ServerName) //nolint:goerr113
			}
			opts := x509.VerifyOptions{
				Roots:         cp,
				Intermediates: x509.NewCertPool(),
			}
			for _, cert := range cs.PeerCertificates[1:] {
				opts.Intermediates.AddCert(cert)
			}
			_, err := cs.PeerCertificates[0].Verify(opts)
			if err != nil {
				return fmt.Errorf("verify peer cert: %w", err)
			}
			return nil
		},
	}

	c.logger.Debug().Str("CN", cn).Msg("setting tls CN")

	return tlsConfig, cn, nil
}

func (c *Check) getBrokerCN(reverseURL *url.URL) (string, error) {
	host := reverseURL.Hostname()

	// OK...
	//
	// mtev_reverse can have an IP or an FQDN for the host portion
	// it used to be that when it was an IP, the CN was needed in order to verify TLS connections
	// otherwise, the FQDN was valid. now, the FQDN may be valid for the cert or it may not be...

	cn := ""

	for _, detail := range c.broker.Details {
		if detail.Status != StatusActive {
			continue
		}
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
		return "", fmt.Errorf("%s: %w", host, errBrokerMatchRevURLHost)
	}

	return cn, nil
}

func (c *Check) fetchBrokerCA() ([]byte, error) {
	// use local file if specified
	file := viper.GetString(config.KeyReverseBrokerCAFile)
	if file != "" {
		cert, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read file: %w", err)
		}
		return cert, nil
	}

	// otherwise, try the api
	data, err := c.client.Get("/pki/ca.crt")
	if err != nil {
		return nil, fmt.Errorf("fetching Broker CA certificate: %w", err)
	}

	type cacert struct {
		Contents string `json:"contents"`
	}

	var cadata cacert

	if err := json.Unmarshal(data, &cadata); err != nil {
		return nil, fmt.Errorf("json parse - Broker CA certificate: %w", err)
	}

	if cadata.Contents == "" {
		return nil, fmt.Errorf("no Broker CA certificate in response (%#v)", string(data)) //nolint:goerr113
	}

	return []byte(cadata.Contents), nil
}
