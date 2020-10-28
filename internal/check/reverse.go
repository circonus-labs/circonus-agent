// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package check

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
)

func (c *Check) setReverseConfigs() error {
	c.revConfigs = nil
	if c.broker == nil {
		return errors.New("broker is uninitialized")
	}
	if c.checkConfig == nil {
		return errors.New("check is uninitialized")
	}

	if len(c.checkConfig.ReverseURLs) == 0 {
		return errors.New("no reverse URLs found in check")
	}

	cfgs := make(ReverseConfigs)

	for _, rURL := range c.checkConfig.ReverseURLs {
		// Replace protocol, url.Parse does not understand 'mtev_reverse'.
		// Important part is validating what's after 'proto://'.
		// Using raw tls connections, the url protocol is not germane.
		reverseURL, err := url.Parse(strings.Replace(rURL, "mtev_reverse", "https", -1))
		if err != nil {
			return errors.Wrapf(err, "parsing check reverse URL (%s)", rURL)
		}

		brokerAddr, err := net.ResolveTCPAddr("tcp", reverseURL.Host)
		if err != nil {
			return errors.Wrapf(err, "invalid reverse service address (%s)", rURL)
		}

		tlsConfig, cn, err := c.brokerTLSConfig(reverseURL)
		if err != nil {
			return errors.Wrapf(err, "creating TLS config for (%s - %s)", c.broker.CID, rURL)
		}

		cfgs[cn] = ReverseConfig{
			CN:         cn,
			ReverseURL: reverseURL,
			BrokerID:   c.broker.CID,
			BrokerAddr: brokerAddr,
			TLSConfig:  tlsConfig,
		}

		c.logger.Debug().
			Str("CN", cn).
			Str("reverse_url", reverseURL.String()).
			Str("broker_id", c.broker.CID).
			Bool("tls", tlsConfig != nil).
			Msg("added reverse config")
	}

	c.revConfigs = &cfgs

	return nil
}

// FindPrimaryBrokerInstance will walk through reverse urls to locate the instance
// in a broker cluster which is the current check owner. Returns the instance cn or error.
func (c *Check) FindPrimaryBrokerInstance(ctx context.Context, cfgs *ReverseConfigs) (string, error) {
	c.Lock()
	defer c.Unlock()

	// there is only one reverse url, broker is not clustered
	if len(*cfgs) == 1 {
		c.logger.Debug().Msg("non-clustered broker identified")
		for name := range *cfgs {
			return name, nil
		}
	}

	primaryCN := ""
	c.logger.Debug().Msg("clustered broker identified, determining which owns check")
	// clustered brokers, need to identify which broker is the primary for the check
	for name, cfg := range *cfgs {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// NOTE: so client doesn't automatically try to connect to the
				// 'Location' returned in the response header. Need to process
				// it not "go" to it.
				return http.ErrUseLastResponse
			},
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

		req, err := http.NewRequestWithContext(ctx, "GET", ownerReqURL, nil)
		if err != nil {
			c.logger.Warn().Err(err).Str("url", ownerReqURL).Msg("creating check owner request")
			return "", err
		}
		req.Header.Add("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			c.logger.Warn().Err(err).Str("url", ownerReqURL).Msg("executing check owner request")
			if nerr, ok := err.(net.Error); ok { //nolint:errorlint
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
		case http.StatusFound:
			location := resp.Header.Get("Location")
			if location == "" {
				c.logger.Warn().Str("req_url", ownerReqURL).Msg("received 302 but 'Location' header missing/blank")
				continue
			}
			c.logger.Debug().Str("location", location).Msg("received Location header")
			// NOTE: this isn't actually a URL, the 'host' portion is actually the CN of
			//       the broker detail which should be used for the reverse connection.
			pu, err := url.Parse(strings.Replace(location, "mtev_reverse", "https", 1))
			if err != nil {
				c.logger.Warn().Err(err).Str("location", location).Msg("unable to parse location")
				continue
			}
			primaryCN = pu.Hostname() // host w/o port...
			c.logger.Debug().Str("cn", primaryCN).Msg("using owner from location header")
		default:
			// try next reverse url host (e.g. if there was an error connecting to this one)
		}
		if primaryCN != "" {
			break
		}
	}

	if primaryCN == "" {
		return "", &ErrNoOwnerFound{
			Err:     "unable to locate check owner broker instance",
			CheckID: c.checkConfig.CID,
		}
	}

	if _, ok := (*cfgs)[primaryCN]; !ok {
		return "", &ErrInvalidOwner{
			Err:      "broker owner identified with invalid CN",
			CheckID:  c.checkConfig.CID,
			BrokerCN: primaryCN,
		}
	}

	c.logger.Debug().Str("cn", primaryCN).Msg("check owner broker instance")
	return primaryCN, nil
}
