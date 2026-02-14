// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	k0stestutil "github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/autopilot/client"
	"github.com/k0sproject/k0s/pkg/kubernetes"

	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NewFakeClientFactory creates new client factory
//
// Deprecated: Use [k0stestutil.NewFakeClientFactory] instead.
func NewFakeClientFactory(objects ...runtime.Object) *FakeClientFactory {
	return &FakeClientFactory{
		FakeClientFactory: k0stestutil.NewFakeClientFactory(objects...),
	}
}

type FakeClientFactory struct {
	*k0stestutil.FakeClientFactory
}

var _ client.FactoryInterface = (*FakeClientFactory)(nil)

// Deprecated: Use [fakeClientFactory.GetAPIExtensionsClient] instead.
func (f FakeClientFactory) GetExtensionClient() (extclient.ApiextensionsV1Interface, error) {
	client, err := f.GetAPIExtensionsClient()
	if err != nil {
		return nil, err
	}

	return client.ApiextensionsV1(), nil
}

// Implements [client.FactoryInterface]: Returns the backing client factory.
func (f *FakeClientFactory) Unwrap() kubernetes.ClientFactoryInterface {
	return f.FakeClientFactory
}
