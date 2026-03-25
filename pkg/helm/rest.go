// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"errors"
	"fmt"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/api/meta"
	_ "k8s.io/cli-runtime/pkg/genericclioptions" // for godoc
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type restClientGetter struct {
	config    *rest.Config
	namespace string

	discoveryClient atomic.Pointer[discovery.CachedDiscoveryInterface]
	restMapper      atomic.Pointer[restmapper.DeferredDiscoveryRESTMapper]
}

// ToRESTConfig implements [genericclioptions.RESTClientGetter].
func (g *restClientGetter) ToRESTConfig() (*rest.Config, error) {
	return rest.CopyConfig(g.config), nil
}

// ToDiscoveryClient implements [genericclioptions.RESTClientGetter].
func (g *restClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	if clientPtr := g.discoveryClient.Load(); clientPtr != nil {
		return *clientPtr, nil
	}

	client, err := discovery.NewDiscoveryClientForConfig(g.config)
	if err != nil {
		return nil, err
	}
	cachedClient := memory.NewMemCacheClient(client)

	if !g.discoveryClient.CompareAndSwap(nil, &cachedClient) {
		cachedClient = *g.discoveryClient.Load()
	}

	return cachedClient, nil
}

// ToRESTMapper implements [genericclioptions.RESTClientGetter].
func (g *restClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	if m := g.restMapper.Load(); m != nil {
		return m, nil
	}

	discoveryClient, err := g.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}

	m := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	if !g.restMapper.CompareAndSwap(nil, m) {
		m = g.restMapper.Load()
	}

	return m, nil
}

// ToRawKubeConfigLoader implements [genericclioptions.RESTClientGetter].
func (g *restClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return g
}

// ConfigAccess implements [clientcmd.ClientConfig].
func (g *restClientGetter) ClientConfig() (*rest.Config, error) {
	return rest.CopyConfig(g.config), nil
}

// ConfigAccess implements [clientcmd.ClientConfig].
func (g *restClientGetter) ConfigAccess() clientcmd.ConfigAccess {
	return &clientcmd.PathOptions{
		LoadingRules: &clientcmd.ClientConfigLoadingRules{},
	}
}

// Namespace implements [clientcmd.ClientConfig].
func (g *restClientGetter) Namespace() (string, bool, error) {
	return g.namespace, true, nil
}

// RawConfig implements [clientcmd.ClientConfig].
func (g *restClientGetter) RawConfig() (api.Config, error) {
	return api.Config{}, fmt.Errorf("%w: RawConfig", errors.ErrUnsupported)
}
