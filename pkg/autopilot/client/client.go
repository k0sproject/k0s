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
	"sync"

	apclient "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2/clientset"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// FactoryInterface is a collection of kubernetes clientset interfaces.
type FactoryInterface interface {
	GetClient() (kubernetes.Interface, error)
	GetAutopilotClient() (apclient.Interface, error)
	GetExtensionClient() (extclient.ApiextensionsV1Interface, error)
	RESTConfig() *rest.Config
}

type clientFactory struct {
	client           kubernetes.Interface
	clientAutopilot  apclient.Interface
	clientExtensions extclient.ApiextensionsV1Interface
	restConfig       *rest.Config

	mutex sync.Mutex
}

var _ FactoryInterface = (*clientFactory)(nil)

func NewClientFactory(config *rest.Config) (FactoryInterface, error) {
	return &clientFactory{restConfig: config}, nil
}

// GetClient returns the core kubernetes clientset
func (cf *clientFactory) GetClient() (kubernetes.Interface, error) {
	cf.mutex.Lock()
	defer cf.mutex.Unlock()
	var err error

	if cf.client != nil {
		return cf.client, nil
	}

	client, err := kubernetes.NewForConfig(cf.restConfig)
	if err != nil {
		return nil, err
	}

	cf.client = client

	return cf.client, nil
}

// GetAutopilotClient returns the clientset for autopilot
func (cf *clientFactory) GetAutopilotClient() (apclient.Interface, error) {
	cf.mutex.Lock()
	defer cf.mutex.Unlock()
	var err error

	if cf.clientAutopilot != nil {
		return cf.clientAutopilot, nil
	}

	client, err := apclient.NewForConfig(cf.restConfig)
	if err != nil {
		return nil, err
	}

	cf.clientAutopilot = client

	return cf.clientAutopilot, nil
}

// GetExtensionClient returns the clientset for kubernetes extensions
func (cf *clientFactory) GetExtensionClient() (extclient.ApiextensionsV1Interface, error) {
	cf.mutex.Lock()
	defer cf.mutex.Unlock()
	var err error

	if cf.clientExtensions != nil {
		return cf.clientExtensions, nil
	}

	client, err := extclient.NewForConfig(cf.restConfig)
	if err != nil {
		return nil, err
	}

	cf.clientExtensions = client

	return cf.clientExtensions, nil
}

func (cf *clientFactory) RESTConfig() *rest.Config {
	return cf.restConfig
}
