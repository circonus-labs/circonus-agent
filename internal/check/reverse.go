// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"net"
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

func (c *Check) setReverseConfig() error {
	if len(c.bundle.ReverseConnectURLs) == 0 {
		return errors.New("no reverse URLs found in check bundle")
	}
	rURL := c.bundle.ReverseConnectURLs[0]
	rSecret := c.bundle.Config["reverse:secret_key"]

	if rSecret != "" {
		rURL += "#" + rSecret
	}

	// Replace protocol, url.Parse does not understand 'mtev_reverse'.
	// Important part is validating what's after 'proto://'.
	// Using raw tls connections, the url protocol is not germane.
	reverseURL, err := url.Parse(strings.Replace(rURL, "mtev_reverse", "http", -1))
	if err != nil {
		return errors.Wrapf(err, "parsing check bundle reverse URL (%s)", rURL)
	}

	brokerAddr, err := net.ResolveTCPAddr("tcp", reverseURL.Host)
	if err != nil {
		return errors.Wrapf(err, "invalid reverse service address (%s)", rURL)
	}

	if len(c.bundle.Brokers) == 0 {
		return errors.New("no brokers found in check bundle")
	}
	brokerID := c.bundle.Brokers[0]

	tlsConfig, err := c.brokerTLSConfig(brokerID, reverseURL)
	if err != nil {
		return errors.Wrapf(err, "creating TLS config for (%s - %s)", brokerID, rURL)
	}

	c.revConfig = &ReverseConfig{
		ReverseURL: reverseURL,
		BrokerID:   brokerID,
		BrokerAddr: brokerAddr,
		TLSConfig:  tlsConfig,
	}

	return nil
}
