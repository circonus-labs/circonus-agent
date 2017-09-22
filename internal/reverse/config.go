// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package reverse

import (
	"crypto/tls"
	"net/url"

	"github.com/pkg/errors"
)

/*
   1. determine check to use:
        i. have check bundle id
        ii. can find with check type and target
   2. fetch check for mtev_reverse url and broker
   3. configure tls
        i. broker ca cert
        ii. broker cn
        iii. tls.Config
*/
func (c *Connection) configure() (*url.URL, *tls.Config, error) {

	bid, reverseURL, err := c.getCheckConfig()
	if err != nil {
		return nil, nil, errors.Wrap(err, "reverse configuration (check)")
	}

	tlsConfig, err := c.getTLSConfig(bid, reverseURL)
	if err != nil {
		return nil, nil, errors.Wrap(err, "reverse configuration (tls)")
	}

	return reverseURL, tlsConfig, nil
}
