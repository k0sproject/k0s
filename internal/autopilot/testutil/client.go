// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	k0stestutil "github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/autopilot/client"
	"github.com/k0sproject/k0s/pkg/kubernetes"

	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
)

// NewFakeClientFactory creates new client factory
//
// Deprecated: Use [k0stestutil.NewFakeClientFactory] instead.
func NewFakeClientFactory() client.FactoryInterface {
	return &fakeClientFactory{
		FakeClientFactory: k0stestutil.NewFakeClientFactory(),
	}
}

type fakeClientFactory struct {
	*k0stestutil.FakeClientFactory
}

var _ client.FactoryInterface = (*fakeClientFactory)(nil)

// Deprecated: Use [fakeClientFactory.GetAPIExtensionsClient] instead.
func (f fakeClientFactory) GetExtensionClient() (extclient.ApiextensionsV1Interface, error) {
	client, err := f.GetAPIExtensionsClient()
	if err != nil {
		return nil, err
	}

	return client.ApiextensionsV1(), nil
}

// Implements [client.FactoryInterface]: Returns the backing client factory.
func (f *fakeClientFactory) Unwrap() kubernetes.ClientFactoryInterface {
	return f.FakeClientFactory
}
