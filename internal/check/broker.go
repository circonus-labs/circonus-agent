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
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// brokerTLSConfig returns the correct TLS configuration for the broker
func (c *Check) brokerTLSConfig(reverseURL *url.URL) (*tls.Config, string, error) {
	if c.broker == nil {
		return nil, "", errors.New("broker not initialized")
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
		return nil, "", errors.New("unable to add Broker CA Certificate to x509 cert pool")
	}

	tlsConfig := &tls.Config{
		// RootCAs:    cp, // see VerifyConnection below - until CN added to SAN in broker certs
		ServerName: cn,
		MinVersion: tls.VersionTLS12,
		// NOTE: This does NOT disable VerifyConnection()
		InsecureSkipVerify: true, //nolint:gosec
		VerifyConnection: func(cs tls.ConnectionState) error {
			commonName := cs.PeerCertificates[0].Subject.CommonName
			if commonName != cs.ServerName {
				return fmt.Errorf("invalid certificate name %q, expected %q", commonName, cs.ServerName)
			}
			opts := x509.VerifyOptions{
				Roots:         cp,
				Intermediates: x509.NewCertPool(),
			}
			for _, cert := range cs.PeerCertificates[1:] {
				opts.Intermediates.AddCert(cert)
			}
			_, err := cs.PeerCertificates[0].Verify(opts)
			return err
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
