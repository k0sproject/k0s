/*
Copyright 2020 k0s authors

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

	cfgClient "github.com/k0sproject/k0s/pkg/client/clientset/typed/k0s/v1beta1"
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
	GetConfigClient() (cfgClient.ClusterConfigInterface, error)
	GetRESTClient() (rest.Interface, error)
	GetRESTConfig() *rest.Config
}

// NewAdminClientFactory creates a new factory that loads the admin kubeconfig based client
func NewAdminClientFactory(kubeconfigPath string) ClientFactoryInterface {
	return &ClientFactory{
		configPath: kubeconfigPath,
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
	configClient    cfgClient.ClusterConfigInterface

	mutex sync.Mutex
}

func (c *ClientFactory) GetClient() (kubernetes.Interface, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	var err error

	if c.restConfig == nil {
		c.restConfig, err = clientcmd.BuildConfigFromFlags("", c.configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
		// We're always running the client on the same host as the API, no need to compress
		c.restConfig.DisableCompression = true
		// To mitigate stack applier bursts in startup
		c.restConfig.QPS = 40.0
		c.restConfig.Burst = 400.0
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
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
		// We're always running the client on the same host as the API, no need to compress
		c.restConfig.DisableCompression = true
		// To mitigate stack applier bursts in startup
		c.restConfig.QPS = 40.0
		c.restConfig.Burst = 400.0
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

func (c *ClientFactory) GetConfigClient() (cfgClient.ClusterConfigInterface, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	var err error
	if c.restConfig == nil {
		c.restConfig, err = clientcmd.BuildConfigFromFlags("", c.configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	}
	if c.configClient != nil {
		return c.configClient, nil
	}

	configClient, err := cfgClient.NewForConfig(c.restConfig)
	if err != nil {
		return nil, err
	}
	c.configClient = configClient.ClusterConfigs(constant.ClusterConfigNamespace)
	return c.configClient, nil
}

func (c *ClientFactory) GetRESTClient() (rest.Interface, error) {
	cs, ok := c.client.(*kubernetes.Clientset)
	if !ok {
		return nil, fmt.Errorf("error converting interface")
	}
	return cs.RESTClient(), nil
}

func (c *ClientFactory) GetRESTConfig() *rest.Config {
	return c.restConfig
}

// KubeconfigFromFile returns a [clientcmd.KubeconfigGetter] that tries to load
// a kubeconfig from the given path.
func KubeconfigFromFile(path string) clientcmd.KubeconfigGetter {
	return (&clientcmd.ClientConfigLoadingRules{ExplicitPath: path}).Load
}

// NewClientFromFile creates a new Kubernetes client based of the given
// kubeconfig file.
func NewClientFromFile(kubeconfig string) (kubernetes.Interface, error) {
	return NewClient(KubeconfigFromFile(kubeconfig))
}

func ClientConfig(getter clientcmd.KubeconfigGetter) (*rest.Config, error) {
	kubeconfig, err := getter()
	if err != nil {
		return nil, err
	}

	return clientcmd.NewNonInteractiveClientConfig(*kubeconfig, "", nil, nil).ClientConfig()
}

// NewClient creates new k8s client based of the given kubeconfig getter. This
// should be only used in cases where the client is "short-running" and
// shouldn't/cannot use the common "cached" one.
func NewClient(getter clientcmd.KubeconfigGetter) (kubernetes.Interface, error) {
	config, err := ClientConfig(getter)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}
