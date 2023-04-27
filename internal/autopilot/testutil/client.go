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
	apclient "github.com/k0sproject/k0s/pkg/client/clientset"
	apclientfake "github.com/k0sproject/k0s/pkg/client/clientset/fake"
	extclientfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"

	"github.com/k0sproject/k0s/pkg/autopilot/client"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

type FakeClientOpt func(config fakeClientConfig) fakeClientConfig

type fakeClientConfig struct {
	kubeObjects      []runtime.Object
	autopilotObjects []runtime.Object
	extensionObjects []runtime.Object
}

// WithKubeObjects seeds a number of objects for use with the core kubernetes clientset
func WithKubeObjects(objects ...runtime.Object) FakeClientOpt {
	return func(config fakeClientConfig) fakeClientConfig {
		config.kubeObjects = objects
		return config
	}
}

// WithAutopilotObjects seeds a number of objects for use with the autopilot clientset
func WithAutopilotObjects(objects ...runtime.Object) FakeClientOpt {
	return func(config fakeClientConfig) fakeClientConfig {
		config.autopilotObjects = objects
		return config
	}
}

// NewFakeClientFactory creates new client factory
func NewFakeClientFactory(opts ...FakeClientOpt) client.FactoryInterface {
	config := fakeClientConfig{}
	for _, opt := range opts {
		config = opt(config)
	}

	return &fakeClientFactory{
		client:           fake.NewSimpleClientset(config.kubeObjects...),
		clientAutopilot:  apclientfake.NewSimpleClientset(config.autopilotObjects...),
		clientExtensions: extclientfake.NewSimpleClientset(config.extensionObjects...).ApiextensionsV1(),
	}
}

type fakeClientFactory struct {
	client           kubernetes.Interface
	clientAutopilot  apclient.Interface
	clientExtensions extclient.ApiextensionsV1Interface
}

var _ client.FactoryInterface = (*fakeClientFactory)(nil)

func (f fakeClientFactory) GetClient() (kubernetes.Interface, error) {
	return f.client, nil
}

func (f fakeClientFactory) GetAutopilotClient() (apclient.Interface, error) {
	return f.clientAutopilot, nil
}

func (f fakeClientFactory) GetExtensionClient() (extclient.ApiextensionsV1Interface, error) {
	return f.clientExtensions, nil
}

func (f fakeClientFactory) RESTConfig() *rest.Config {
	return nil
}
