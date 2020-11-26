/*
Copyright 2020 Mirantis, Inc.

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
package kubernetes

import (
	"sync"

	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ClientFactory defines a factory interface to load a kube client
type ClientFactory interface {
	Create() (kubernetes.Interface, error)
}

// NewAdminClientFactory creates a new factory that loads the admin kubeconfig based client
func NewAdminClientFactory(k0sVars constant.CfgVars) ClientFactory {
	return &clientFactory{
		configPath: k0sVars.AdminKubeConfigPath,
	}
}

type clientFactory struct {
	configPath string

	client kubernetes.Interface
	mutex  sync.Mutex
}

func (c *clientFactory) Create() (kubernetes.Interface, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.client != nil {
		return c.client, nil
	}

	client, err := Client(c.configPath)
	if err != nil {
		return nil, err
	}

	c.client = client

	return c.client, nil

}

// Client creates new k8s client based of the given kubeconfig
func Client(kubeconfig string) (kubernetes.Interface, error) {
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load kubeconfig")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create k8s client")
	}

	return clientset, nil
}
