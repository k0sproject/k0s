/*
Copyright 2021 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package token

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"k8s.io/client-go/tools/clientcmd"
)

// JoinClient is the client we can use to call k0s join APIs
type JoinClient struct {
	joinAddress   string
	httpClient    http.Client
	bearerToken   string
	joinTokenType string
}

// JoinClientFromToken creates a new join api client from a token
func JoinClientFromToken(encodedToken string) (*JoinClient, error) {
	tokenBytes, err := DecodeJoinToken(encodedToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decode token: %w", err)
	}

	clientConfig, err := clientcmd.NewClientConfigFromBytes(tokenBytes)
	if err != nil {
		return nil, err
	}
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	raw, err := clientConfig.RawConfig()
	if err != nil {
		return nil, err
	}

	ca := x509.NewCertPool()
	ca.AppendCertsFromPEM(config.CAData)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		RootCAs:            ca,
	}
	tr := &http.Transport{TLSClientConfig: tlsConfig}
	c := &JoinClient{
		httpClient:  http.Client{Transport: tr},
		bearerToken: config.BearerToken,
	}
	c.joinAddress = config.Host
	c.joinTokenType = GetTokenType(&raw)

	return c, nil
}

func (j *JoinClient) Address() string {
	return j.joinAddress
}

func (j *JoinClient) JoinTokenType() string {
	return j.joinTokenType
}

// GetCA calls the CA sync API
func (j *JoinClient) GetCA(ctx context.Context) (v1beta1.CaResponse, error) {
	var caData v1beta1.CaResponse
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, j.joinAddress+"/v1beta1/ca", nil)
	if err != nil {
		return caData, fmt.Errorf("failed to create join request: %w", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", j.bearerToken))

	resp, err := j.httpClient.Do(req)
	if err != nil {
		return caData, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return caData, fmt.Errorf("unexpected response status: %s", resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return caData, err
	}
	err = json.Unmarshal(b, &caData)
	if err != nil {
		return caData, err
	}
	return caData, nil
}

// JoinEtcd calls the etcd join API
func (j *JoinClient) JoinEtcd(ctx context.Context, etcdRequest v1beta1.EtcdRequest) (v1beta1.EtcdResponse, error) {
	var etcdResponse v1beta1.EtcdResponse
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(etcdRequest); err != nil {
		return etcdResponse, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, j.joinAddress+"/v1beta1/etcd/members", buf)
	if err != nil {
		return etcdResponse, fmt.Errorf("failed to create join request: %w", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", j.bearerToken))
	resp, err := j.httpClient.Do(req)
	if err != nil {
		return etcdResponse, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return etcdResponse, fmt.Errorf("unexpected response status when trying to join etcd cluster: %s", resp.Status)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return etcdResponse, err
	}
	err = json.Unmarshal(b, &etcdResponse)
	if err != nil {
		return etcdResponse, err
	}

	return etcdResponse, nil
}
