// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
	joinAddress string
	restClient  *rest.RESTClient
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

	if actual := GetTokenType(kubeconfig); actual != ControllerTokenAuthName {
		return nil, fmt.Errorf("wrong token type %s, expected type: %s", actual, ControllerTokenAuthName)
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
		joinAddress: restConfig.Host,
		restClient:  restClient,
	}, nil
}

func (j *JoinClient) Address() string {
	return j.joinAddress
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
