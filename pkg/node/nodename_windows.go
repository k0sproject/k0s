// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/k0sproject/k0s/pkg/k0scontext"
)

// A URL that may be retrieved to determine the nodename.
type nodenameURL string

func defaultNodeNameOverride(ctx context.Context) (string, error) {
	// we need to check if we have EC2 dns name available
	url := k0scontext.ValueOr[nodenameURL](ctx, "http://169.254.169.254/latest/meta-data/local-hostname")

	client := http.Client{
		Transport: &http.Transport{DisableKeepAlives: true},
		Timeout:   1 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, string(url), nil)
	if err != nil {
		return "", err
	}

	// if status code is 2XX we assume we are running on ec2
	if resp, err := client.Do(req); err == nil {
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
			if bytes, err := io.ReadAll(resp.Body); err == nil {
				return string(bytes), nil
			}
		}
	}

	return "", nil
}
