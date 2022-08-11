// Copyright © 2018 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (c *Client) get(ctx context.Context, reqpath string) ([]byte, error) {
	if reqpath == "" {
		return nil, errInvalidRequestPath
	}

	au, err := c.agentURL.Parse(reqpath)
	if err != nil {
		return nil, fmt.Errorf("creating request URL: %w", err)
	}

	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, "GET", au.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("preparing reqeust: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request (%s): %w", au.String(), err)
	}

	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s - %s - %s: %w", resp.Status, au.String(), strings.TrimSpace(string(data)), errInvalidHTTPResponse)
	}

	return data, nil
}
