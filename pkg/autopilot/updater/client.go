// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/k0sproject/k0s/internal/secret"

	"sigs.k8s.io/yaml"
)

const defaultChannel = "stable"

type Client interface {
	GetUpdate(channel, clusterID, lastUpdateStatus, k0sVersion string) (*Update, error)
}

type client struct {
	httpClient      *http.Client
	updateServer    string
	updateServerURL *url.URL
	authToken       secret.String
}

// NewClient creates a client for the given update server. The auth token is
// sent as bearer token; the zero token leaves requests unauthenticated.
func NewClient(updateServer string, authToken secret.String) (Client, error) {
	url, err := url.Parse(updateServer)
	if err != nil {
		return nil, err
	}
	c := &client{
		httpClient:      http.DefaultClient,
		updateServer:    updateServer,
		updateServerURL: url,
		authToken:       authToken,
	}
	return c, nil
}

func (c *client) GetUpdate(channel, clusterID, lastUpdateStatus, currentVersion string) (*Update, error) {
	if channel == "" {
		channel = defaultChannel
	}
	u := *c.updateServerURL
	u.Path = channel
	query := url.Values{}
	query.Set("clusterID", clusterID)
	query.Set("lastUpdateStatus", lastUpdateStatus)
	query.Set("currentVersion", currentVersion)
	u.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	if token, err := c.authToken.Reveal(); err == nil {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received unexpected status (%d) from update server", resp.StatusCode)
	}
	var update Update

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(body, &update)
	if err != nil {
		return nil, err
	}

	return &update, nil
}
