// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

func (c *Client) Write(groupID string, metrics *Metrics) error {
	return c.WriteWithContext(context.Background(), groupID, metrics)
}

func (c *Client) WriteWithContext(ctx context.Context, groupID string, metrics *Metrics) error {
	if groupID == "" {
		return errors.New("invalid group id (empty)")
	}
	if metrics == nil {
		return errors.New("invalid metrics (nil)")
	}
	if len(*metrics) == 0 {
		return errors.New("invalid metrics (none)")
	}

	au, err := c.agentURL.Parse("/write/" + groupID)
	if err != nil {
		return errors.Wrap(err, "creating request url")
	}

	m, err := json.Marshal(metrics)
	if err != nil {
		return errors.Wrap(err, "converting metrics to JSON")
	}

	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, "POST", au.String(), bytes.NewBuffer(m))
	if err != nil {
		return errors.Wrap(err, "preparing request")
	}

	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "request")
	}

	switch resp.StatusCode {
	case http.StatusOK:
		fallthrough
	case http.StatusNoContent:
		return nil // good, metrics were accepted
	default:
		// extract any error message and return
		defer resp.Body.Close()
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return errors.Wrap(err, "reading response")
		}

		return errors.Errorf("%s - %s - %s", resp.Status, au.String(), strings.TrimSpace(string(data)))
	}
}
