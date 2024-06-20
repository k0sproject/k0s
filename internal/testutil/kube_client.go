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

package testutil

import (
	"fmt"

	"k8s.io/client-go/rest"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	restfake "k8s.io/client-go/rest/fake"
	kubetesting "k8s.io/client-go/testing"

	etcdMemberClient "github.com/k0sproject/k0s/pkg/client/clientset/typed/etcd/v1beta1"
	cfgClient "github.com/k0sproject/k0s/pkg/client/clientset/typed/k0s/v1beta1"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
)

var _ kubeutil.ClientFactoryInterface = (*FakeClientFactory)(nil)

// NewFakeClientFactory creates new client factory which uses internally only the kube fake client interface
func NewFakeClientFactory(objects ...runtime.Object) *FakeClientFactory {
	scheme := kubernetesscheme.Scheme

	rawDiscovery := &discoveryfake.FakeDiscovery{Fake: &kubetesting.Fake{}}

	return &FakeClientFactory{
		Client:          fake.NewSimpleClientset(objects...),
		DynamicClient:   dynamicfake.NewSimpleDynamicClient(scheme),
		DiscoveryClient: memory.NewMemCacheClient(rawDiscovery),
		RawDiscovery:    rawDiscovery,
		RESTClient:      &restfake.RESTClient{},
	}
}

type FakeClientFactory struct {
	Client          kubernetes.Interface
	DynamicClient   dynamic.Interface
	DiscoveryClient discovery.CachedDiscoveryInterface
	RawDiscovery    *discoveryfake.FakeDiscovery
	RESTClient      rest.Interface
}

func (f *FakeClientFactory) GetClient() (kubernetes.Interface, error) {
	return f.Client, nil
}

func (f *FakeClientFactory) GetDynamicClient() (dynamic.Interface, error) {
	return f.DynamicClient, nil
}

func (f *FakeClientFactory) GetDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return f.DiscoveryClient, nil
}

func (f *FakeClientFactory) GetConfigClient() (cfgClient.ClusterConfigInterface, error) {
	return nil, fmt.Errorf("NOT IMPLEMENTED")
}

func (f *FakeClientFactory) GetRESTClient() (rest.Interface, error) {
	return f.RESTClient, nil
}
func (f *FakeClientFactory) GetRESTConfig() *rest.Config {
	return &rest.Config{}
}

func (f *FakeClientFactory) GetEtcdMemberClient() (etcdMemberClient.EtcdMemberInterface, error) {
	return nil, fmt.Errorf("NOT IMPLEMENTED")
}
