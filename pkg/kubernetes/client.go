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
package kubernetes

import (
	"fmt"
	"sync"

	"github.com/k0sproject/k0s/pkg/constant"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ClientFactoryInterface defines a factory interface to load a kube client
type ClientFactoryInterface interface {
	GetClient() (kubernetes.Interface, error)
	GetDynamicClient() (dynamic.Interface, error)
	GetDiscoveryClient() (discovery.CachedDiscoveryInterface, error)
}

// NewAdminClientFactory creates a new factory that loads the admin kubeconfig based client
func NewAdminClientFactory(k0sVars constant.CfgVars) ClientFactoryInterface {
	return &ClientFactory{
		configPath: k0sVars.AdminKubeConfigPath,
	}
}

// ClientFactory implements a cached and lazy-loading ClientFactory for all the different types of kube clients we use
// It's imoplemented as lazy-loading so we can create the factory itself before we have the api, etcd and other components up so we can pass
// the factory itself to components needing kube clients and creation time.
type ClientFactory struct {
	configPath string

	client          kubernetes.Interface
	dynamicClient   dynamic.Interface
	discoveryClient discovery.CachedDiscoveryInterface
	restConfig      *rest.Config

	mutex sync.Mutex
}

func (c *ClientFactory) GetClient() (kubernetes.Interface, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	var err error

	if c.restConfig == nil {
		c.restConfig, err = clientcmd.BuildConfigFromFlags("", c.configPath)
		// We're always running the client on the same host as the API, no need to compress
		c.restConfig.DisableCompression = true
		// To mitigate stack applier bursts in startup
		c.restConfig.QPS = 40.0
		c.restConfig.Burst = 400.0
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	}

	if c.client != nil {
		return c.client, nil
	}

	client, err := kubernetes.NewForConfig(c.restConfig)
	if err != nil {
		return nil, err
	}

	c.client = client

	return c.client, nil
}

func (c *ClientFactory) GetDynamicClient() (dynamic.Interface, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	var err error
	if c.restConfig == nil {
		c.restConfig, err = clientcmd.BuildConfigFromFlags("", c.configPath)
		// We're always running the client on the same host as the API, no need to compress
		c.restConfig.DisableCompression = true
		// To mitigate stack applier bursts in startup
		c.restConfig.QPS = 40.0
		c.restConfig.Burst = 400.0
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	}

	if c.dynamicClient != nil {
		return c.dynamicClient, nil
	}

	dynamicClient, err := dynamic.NewForConfig(c.restConfig)
	if err != nil {
		return nil, err
	}

	c.dynamicClient = dynamicClient

	return c.dynamicClient, nil
}

func (c *ClientFactory) GetDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	var err error
	if c.restConfig == nil {
		c.restConfig, err = clientcmd.BuildConfigFromFlags("", c.configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	}

	if c.discoveryClient != nil {
		return c.discoveryClient, nil
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(c.restConfig)
	cachedDiscoveryClient := memory.NewMemCacheClient(discoveryClient)
	if err != nil {
		return nil, err
	}
	c.discoveryClient = cachedDiscoveryClient

	return c.discoveryClient, nil
}

// NewClient creates new k8s client based of the given kubeconfig
// This should be only used in cases where the client is "short-running" and shouldn't/cannot use the common "cached" one.
func NewClient(kubeconfig string) (kubernetes.Interface, error) {
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	return clientset, nil
}
