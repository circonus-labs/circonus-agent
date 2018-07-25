// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package api

import (
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

func (c *Client) get(reqpath string) ([]byte, error) {
	if reqpath == "" {
		return nil, errors.New("invalid request path (empty)")
	}

	au, err := c.agentURL.Parse(reqpath)
	if err != nil {
		return nil, errors.Wrap(err, "creating request url")
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", au.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "preparing request")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "request")
	}

	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "reading response")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("%s - %s - %s", resp.Status, au.String(), strings.TrimSpace(string(data)))
	}

	return data, nil
}
