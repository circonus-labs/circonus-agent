// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (c *Client) Write(groupID string, metrics *Metrics) error {
	return c.WriteWithContext(context.Background(), groupID, metrics)
}

func (c *Client) WriteWithContext(ctx context.Context, groupID string, metrics *Metrics) error {
	if groupID == "" {
		return errInvalidGroupID
	}
	if metrics == nil {
		return errInvalidMetrics
	}
	if len(*metrics) == 0 {
		return errInvalidMetricList
	}

	au, err := c.agentURL.Parse("/write/" + groupID)
	if err != nil {
		return fmt.Errorf("creating request url: %w", err)
	}

	m, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("json encode - metrics: %w", err)
	}

	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, "POST", au.String(), bytes.NewBuffer(m))
	if err != nil {
		return fmt.Errorf("preparing request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		fallthrough
	case http.StatusNoContent:
		return nil // good, metrics were accepted
	default:
		// extract any error message and return
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("reading response: %w", err)
		}

		return fmt.Errorf("%s - %s - %s: %w", resp.Status, au.String(), strings.TrimSpace(string(data)), errInvalidHTTPResponse)
	}
}
