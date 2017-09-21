// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	stdlog "log"
	"net/url"
	"regexp"

	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-gometrics/api"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func getTLSConfig(cid string, reverseURL *url.URL) (*tls.Config, error) {
	if cid == "" {
		return nil, errors.New("No broker CID supplied")
	}
	if ok, _ := regexp.MatchString("^/broker/[0-9]+$", cid); !ok {
		return nil, errors.Errorf("Invalid broker CID (%s)", cid)
	}

	cfg := &api.Config{
		TokenKey: viper.GetString(config.KeyAPITokenKey),
		TokenApp: viper.GetString(config.KeyAPITokenApp),
		URL:      viper.GetString(config.KeyAPIURL),
		Log:      stdlog.New(logger.With().Str("pkg", "circonus-gometrics.api").Logger(), "", 0),
		Debug:    viper.GetBool(config.KeyDebugCGM),
	}

	client, err := api.New(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "Initializing cgm API")
	}

	broker, err := client.FetchBroker(api.CIDType(&cid))
	if err != nil {
		return nil, errors.Wrapf(err, "Fetching broker (%s) from API", cid)
	}

	cn, err := getBrokerCN(broker, reverseURL)
	if err != nil {
		return nil, err
	}
	cert, err := fetchBrokerCA(client)
	if err != nil {
		return nil, err
	}
	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM(cert) {
		return nil, errors.New("Unable to add Broker CA Certificate to x509 cert pool")
	}

	tlsConfig := &tls.Config{
		RootCAs:    cp,
		ServerName: cn,
	}

	logger.Debug().Str("CN", cn).Msg("setting tls CN")

	return tlsConfig, nil
}

func getBrokerCN(broker *api.Broker, reverseURL *url.URL) (string, error) {
	host := reverseURL.Hostname()

	// OK...
	//
	// mtev_reverse can have an IP or an FQDN for the host portion
	// it used to be that when it was an IP, the CN was needed in order to verify TLS connections
	// otherwise, the FQDN was valid. now, the FQDN may be valid for the cert or it may not be...

	cn := ""

	for _, detail := range broker.Details {
		// certs are generated agains the CN (in theory)
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
		return "", errors.Errorf("Unable to match reverse URL host (%s) to broker", host)
	}

	return cn, nil
}

func fetchBrokerCA(client *api.API) ([]byte, error) {
	// use local file if specified
	file := viper.GetString(config.KeyReverseBrokerCAFile)
	if file != "" {
		cert, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, errors.Wrapf(err, "Reading specified broker-ca-file (%s)", file)
		}
		return cert, nil
	}

	// otherwise, try the api
	data, err := client.Get("/pki/ca.crt")
	if err != nil {
		return nil, errors.Wrap(err, "Fetching Broker CA certificate")
	}

	type cacert struct {
		Contents string `json:"contents"`
	}

	var cadata cacert

	if err := json.Unmarshal(data, &cadata); err != nil {
		return nil, errors.Wrap(err, "Parsing Broker CA certificate")
	}

	if cadata.Contents == "" {
		return nil, errors.Errorf("No Broker CA certificate in response (%#v)", string(data))
	}

	return []byte(cadata.Contents), nil
}
