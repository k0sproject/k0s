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
	"sync"

	k0sclientset "github.com/k0sproject/k0s/pkg/client/clientset"
	etcdMemberClient "github.com/k0sproject/k0s/pkg/client/clientset/typed/etcd/v1beta1"
	cfgClient "github.com/k0sproject/k0s/pkg/client/clientset/typed/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
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
	GetAPIExtensionsClient() (apiextensionsclientset.Interface, error)
	GetK0sClient() (k0sclientset.Interface, error)
	GetConfigClient() (cfgClient.ClusterConfigInterface, error) // Deprecated: Use [ClientFactoryInterface.GetK0sClient] instead.
	GetRESTConfig() (*rest.Config, error)
	GetEtcdMemberClient() (etcdMemberClient.EtcdMemberInterface, error) // Deprecated: Use [ClientFactoryInterface.GetK0sClient] instead.
}

// ClientFactory implements a cached and lazy-loading ClientFactory for all the different types of kube clients we use
// It's imoplemented as lazy-loading so we can create the factory itself before we have the api, etcd and other components up so we can pass
// the factory itself to components needing kube clients and creation time.
type ClientFactory struct {
	LoadRESTConfig func() (*rest.Config, error)

	client              kubernetes.Interface
	dynamicClient       dynamic.Interface
	discoveryClient     discovery.CachedDiscoveryInterface
	apiExtensionsClient apiextensionsclientset.Interface
	k0sClient           k0sclientset.Interface
	restConfig          *rest.Config

	mutex sync.Mutex
}

func (c *ClientFactory) GetClient() (kubernetes.Interface, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.client != nil {
		return c.client, nil
	}

	config, err := c.getRESTConfig()
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	c.client = client

	return client, nil
}

func (c *ClientFactory) GetDynamicClient() (dynamic.Interface, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.dynamicClient != nil {
		return c.dynamicClient, nil
	}

	config, err := c.getRESTConfig()
	if err != nil {
		return nil, err
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	c.dynamicClient = client

	return client, nil
}

func (c *ClientFactory) GetDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.discoveryClient != nil {
		return c.discoveryClient, nil
	}

	config, err := c.getRESTConfig()
	if err != nil {
		return nil, err
	}

	client, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	cachedClient := memory.NewMemCacheClient(client)

	c.discoveryClient = cachedClient

	return cachedClient, nil
}

func (c *ClientFactory) GetAPIExtensionsClient() (apiextensionsclientset.Interface, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.apiExtensionsClient != nil {
		return c.apiExtensionsClient, nil
	}

	config, err := c.getRESTConfig()
	if err != nil {
		return nil, err
	}

	client, err := apiextensionsclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	c.apiExtensionsClient = client

	return client, nil
}

func (c *ClientFactory) GetK0sClient() (k0sclientset.Interface, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.k0sClient != nil {
		return c.k0sClient, nil
	}

	config, err := c.getRESTConfig()
	if err != nil {
		return nil, err
	}

	client, err := k0sclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	c.k0sClient = client

	return client, nil
}

// Deprecated: Use [ClientFactory.GetK0sClient] instead.
func (c *ClientFactory) GetConfigClient() (cfgClient.ClusterConfigInterface, error) {
	k0sClient, err := c.GetK0sClient()
	if err != nil {
		return nil, err
	}

	return k0sClient.K0sV1beta1().ClusterConfigs(constant.ClusterConfigNamespace), nil
}

// Deprecated: Use [ClientFactory.GetK0sClient] instead.
func (c *ClientFactory) GetEtcdMemberClient() (etcdMemberClient.EtcdMemberInterface, error) {
	k0sClient, err := c.GetK0sClient()
	if err != nil {
		return nil, err
	}

	return k0sClient.EtcdV1beta1().EtcdMembers(), nil
}

func (c *ClientFactory) GetRESTConfig() (*rest.Config, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.getRESTConfig()
}

func (c *ClientFactory) getRESTConfig() (*rest.Config, error) {
	if c.restConfig != nil {
		return c.restConfig, nil
	}

	config, err := c.LoadRESTConfig()
	if err != nil {
		return nil, err
	}

	c.restConfig = config

	return config, err
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
