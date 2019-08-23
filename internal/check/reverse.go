// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/circonus-labs/go-apiclient"
	"github.com/pkg/errors"
)

func (c *Check) setReverseConfigs() error {
	c.revConfigs = nil

	if len(c.bundle.ReverseConnectURLs) == 0 {
		return errors.New("no reverse URLs found in check bundle")
	}

	// set the check broker
	if len(c.bundle.Brokers) == 0 {
		return errors.New("no brokers found in check bundle")
	}
	brokerID := c.bundle.Brokers[0]

	c.broker = nil
	broker, err := c.client.FetchBroker(apiclient.CIDType(&brokerID))
	if err != nil {
		return errors.Wrapf(err, "unable to retrieve broker (%s)", brokerID)
	}
	c.broker = broker

	cfgs := make(ReverseConfigs)

	for _, rURL := range c.bundle.ReverseConnectURLs {
		rSecret := c.bundle.Config["reverse:secret_key"]

		if rSecret != "" {
			rURL += "#" + rSecret
		}

		// Replace protocol, url.Parse does not understand 'mtev_reverse'.
		// Important part is validating what's after 'proto://'.
		// Using raw tls connections, the url protocol is not germane.
		reverseURL, err := url.Parse(strings.Replace(rURL, "mtev_reverse", "https", -1))
		if err != nil {
			return errors.Wrapf(err, "parsing check bundle reverse URL (%s)", rURL)
		}

		brokerAddr, err := net.ResolveTCPAddr("tcp", reverseURL.Host)
		if err != nil {
			return errors.Wrapf(err, "invalid reverse service address (%s)", rURL)
		}

		tlsConfig, cn, err := c.brokerTLSConfig(brokerID, reverseURL)
		if err != nil {
			return errors.Wrapf(err, "creating TLS config for (%s - %s)", brokerID, rURL)
		}

		cfgs[cn] = ReverseConfig{
			CN:         cn,
			ReverseURL: reverseURL,
			BrokerID:   brokerID,
			BrokerAddr: brokerAddr,
			TLSConfig:  tlsConfig,
		}
	}

	c.revConfigs = &cfgs
	return nil
}

// FindPrimaryBrokerInstance will walk through reverse urls to locate the instance
// in a broker cluster which is the current check owner. Returns the instance cn or error.
func (c *Check) FindPrimaryBrokerInstance(cfgs *ReverseConfigs) (string, error) {
	c.Lock()
	defer c.Unlock()

	primaryHost := ""
	primaryCN := ""

	// there is only one reverse url, broker is not clustered
	if len(*cfgs) == 1 {
		c.logger.Debug().Msg("non-clustered broker identified")
		for name := range *cfgs {
			return name, nil
		}
	}

	c.logger.Debug().Msg("clustered broker identified, determining which owns check")
	// clustered brokers, need to identify which broker is the primary for the check
	for name, cfg := range *cfgs {
		client := &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				Dial: (&net.Dialer{
					Timeout: 5 * time.Second,
				}).Dial,
				TLSHandshakeTimeout: 3 * time.Second,
				TLSClientConfig:     cfg.TLSConfig, // all reverse brokers use HTTPS/TLS
				DisableKeepAlives:   true,
				MaxIdleConnsPerHost: -1,
				DisableCompression:  false,
			},
		}

		ownerReqURL := strings.Replace(cfg.ReverseURL.String(), "/check/", "/checks/owner/", 1)
		c.logger.Debug().Str("trying", ownerReqURL).Msg("checking")

		req, err := http.NewRequest("GET", ownerReqURL, nil)
		if err != nil {
			c.logger.Warn().Err(err).Str("url", ownerReqURL).Msg("creating check owner request")
			return "", err
		}
		req.Header.Add("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			c.logger.Warn().Err(err).Str("url", cfg.ReverseURL.String()).Msg("executing check owner request")
			if nerr, ok := err.(net.Error); ok {
				if nerr.Timeout() {
					continue
				}
			}
			return "", err
		}
		resp.Body.Close() // we only care about headers

		switch resp.StatusCode {
		case http.StatusNoContent:
			primaryCN = name
			c.logger.Debug().Str("cn", primaryCN).Msg("found owner")
			break
		case http.StatusFound:
			location := resp.Header.Get("Location")
			if location == "" {
				c.logger.Warn().Msg("received 302 but 'Location' header missing/blank")
				continue
			}
			c.logger.Debug().Str("location", location).Msg("received Location header")
			// NOTE: this isn't actually a URL, the 'host' portion is actually the CN of
			//       the broker detail which should be used for the reverse connection.
			pu, err := url.Parse(location)
			if err != nil {
				c.logger.Warn().Err(err).Str("location", location).Msg("unable to parse location")
				continue
			}
			primaryHost = pu.Host
			c.logger.Debug().Str("cn", primaryCN).Msg("using owner from location header")
			break
		default:
			// try next reverse url host (e.g. if there was an error connecting to this one)
		}
	}

	if primaryCN == "" && primaryHost != "" {
		for name, cfg := range *cfgs {
			if cfg.ReverseURL.Host == primaryHost {
				primaryCN = name
			}
		}
	}

	if primaryCN == "" {
		return "", &NoOwnerFoundError{Err: "unable to locate check owner broker instance", BundleID: c.bundle.CID}
	}

	c.logger.Debug().Str("cn", primaryCN).Msg("check owner broker instance")
	return primaryCN, nil
}
