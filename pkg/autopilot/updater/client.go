// Copyright 2021 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package updater

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

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
	authToken       string
}

func NewClient(updateServer string, authToken string) (Client, error) {
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

	if c.authToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
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
