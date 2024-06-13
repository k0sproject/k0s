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
	"encoding/json"
	"fmt"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/kubernetes"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// JoinClient is the client we can use to call k0s join APIs
type JoinClient struct {
	joinTokenType string
	joinAddress   string
	restClient    *rest.RESTClient
}

// JoinClientFromToken creates a new join api client from a token
func JoinClientFromToken(encodedToken string) (*JoinClient, error) {
	tokenBytes, err := DecodeJoinToken(encodedToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decode token: %w", err)
	}

	kubeconfig, err := clientcmd.Load(tokenBytes)
	if err != nil {
		return nil, err
	}

	restConfig, err := kubernetes.ClientConfig(func() (*api.Config, error) { return kubeconfig, nil })
	if err != nil {
		return nil, err
	}

	restConfig = dynamic.ConfigFor(restConfig)
	restClient, err := rest.UnversionedRESTClientFor(restConfig)
	if err != nil {
		return nil, err
	}

	return &JoinClient{
		joinAddress:   restConfig.Host,
		joinTokenType: GetTokenType(kubeconfig),
		restClient:    restClient,
	}, nil
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

	b, err := j.restClient.Get().AbsPath("v1beta1", "ca").Do(ctx).Raw()
	if err == nil {
		err = json.Unmarshal(b, &caData)
	}

	return caData, err
}

// JoinEtcd calls the etcd join API
func (j *JoinClient) JoinEtcd(ctx context.Context, etcdRequest v1beta1.EtcdRequest) (v1beta1.EtcdResponse, error) {
	var etcdResponse v1beta1.EtcdResponse

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(etcdRequest); err != nil {
		return etcdResponse, err
	}

	b, err := j.restClient.Post().AbsPath("v1beta1", "etcd", "members").Body(buf).Do(ctx).Raw()
	if err == nil {
		err = json.Unmarshal(b, &etcdResponse)
	}

	return etcdResponse, err
}
