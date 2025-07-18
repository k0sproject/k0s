// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package channels

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"sigs.k8s.io/yaml"
)

type ChannelClient struct {
	httpClient *http.Client
	token      string
	channelURL string
}

func NewChannelClient(server string, channel string, token string) (*ChannelClient, error) {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	// If server is a full URL, use that. If not, assume HTTPS.
	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}
	if serverURL.Scheme == "" {
		serverURL.Scheme = "https"
	}

	channelURL := serverURL.JoinPath(channel, "index.yaml")

	return &ChannelClient{
		httpClient: httpClient,
		token:      token,
		channelURL: channelURL.String(),
	}, nil
}

func (c *ChannelClient) GetLatest(ctx context.Context, headers map[string]string) (VersionInfo, error) {

	var v VersionInfo

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.channelURL, nil)
	if err != nil {
		return v, err
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	if c.token != "" {
		req.Header.Add("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return v, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return v, fmt.Errorf("error fetching channel: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return v, err
	}

	if err := yaml.Unmarshal(data, &v); err != nil {
		return v, err
	}

	return v, nil
}
