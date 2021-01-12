package testutil

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
)

func NewFakeClientFactory() FakeClientFactory {
	rawDiscovery := &discoveryfake.FakeDiscovery{Fake: &kubetesting.Fake{}}

	return FakeClientFactory{
		Client:          fake.NewSimpleClientset(),
		DynamicClient:   dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		DiscoveryClient: memory.NewMemCacheClient(rawDiscovery),
		RawDiscovery:    rawDiscovery,
	}
}

type FakeClientFactory struct {
	Client          kubernetes.Interface
	DynamicClient   dynamic.Interface
	DiscoveryClient discovery.CachedDiscoveryInterface
	RawDiscovery    *discoveryfake.FakeDiscovery
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
