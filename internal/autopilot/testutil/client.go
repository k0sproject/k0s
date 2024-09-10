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

package testutil

import (
	k0stestutil "github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/autopilot/client"
	"github.com/k0sproject/k0s/pkg/kubernetes"

	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
)

// NewFakeClientFactory creates new client factory
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
