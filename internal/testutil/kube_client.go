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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	restfake "k8s.io/client-go/rest/fake"
	kubetesting "k8s.io/client-go/testing"

	cfgClient "github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/clientset/typed/k0s.k0sproject.io/v1beta1"
)

// NewFakeClientFactory creates new client factory which uses internally only the kube fake client interface
func NewFakeClientFactory(objects ...runtime.Object) FakeClientFactory {
	rawDiscovery := &discoveryfake.FakeDiscovery{Fake: &kubetesting.Fake{}}

	// Remember to list all "xyzList" types for resource types we use with the fake client
	// and use "list" verb on
	gvkLists := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}:                                          "PodList",
		{Group: "", Version: "v1", Resource: "namespaces"}:                                    "NamespaceList",
		{Group: "", Version: "v1", Resource: "nodes"}:                                         "NodeList",
		{Group: "", Version: "v1", Resource: "configmaps"}:                                    "ConfigMapList",
		{Group: "certificates.k8s.io", Version: "v1", Resource: "certificatesigningrequests"}: "CertificateSigningRequestList",
		{Group: "apps", Version: "v1", Resource: "deployments"}:                               "DeploymentList",
	}

	return FakeClientFactory{
		Client:          fake.NewSimpleClientset(objects...),
		DynamicClient:   dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvkLists),
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

func (f FakeClientFactory) GetClient() (kubernetes.Interface, error) {
	return f.Client, nil
}

func (f FakeClientFactory) GetDynamicClient() (dynamic.Interface, error) {
	return f.DynamicClient, nil
}

func (f FakeClientFactory) GetDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return f.DiscoveryClient, nil
}

func (f FakeClientFactory) GetConfigClient() (cfgClient.ClusterConfigInterface, error) {
	return nil, fmt.Errorf("NOT IMPLEMENTED")
}

func (f FakeClientFactory) GetRESTClient() (rest.Interface, error) {
	return f.RESTClient, nil
}
func (f FakeClientFactory) GetRESTConfig() *rest.Config {
	return &rest.Config{}
}
