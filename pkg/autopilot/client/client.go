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

package client

import (
	apclient "github.com/k0sproject/k0s/pkg/client/clientset"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// FactoryInterface is a collection of kubernetes clientset interfaces.
// Deprecated: Use [kubeutil.ClientFactoryInterface] instead.
type FactoryInterface interface {
	GetClient() (kubernetes.Interface, error)
	GetK0sClient() (apclient.Interface, error)
	GetAPIExtensionsClient() (apiextensionsclientset.Interface, error)
	// Deprecated: Use [FactoryInterface.GetAPIExtensionsClient] instead.
	GetExtensionClient() (extclient.ApiextensionsV1Interface, error)
	GetRESTConfig() (*rest.Config, error)
	Unwrap() kubeutil.ClientFactoryInterface
}

// Deprecated: Use [kubeutil.ClientFactory] instead.
type ClientFactory struct {
	kubeutil.ClientFactoryInterface
}

var _ FactoryInterface = (*ClientFactory)(nil)

// Deprecated: Use [ClientFactory.GetAPIExtensionsClient] instead.
func (f *ClientFactory) GetExtensionClient() (extclient.ApiextensionsV1Interface, error) {
	client, err := f.GetAPIExtensionsClient()
	if err != nil {
		return nil, err
	}

	return client.ApiextensionsV1(), nil
}

// Implements [FactoryInterface]: Returns the backing client factory.
func (f *ClientFactory) Unwrap() kubeutil.ClientFactoryInterface {
	return f.ClientFactoryInterface
}
