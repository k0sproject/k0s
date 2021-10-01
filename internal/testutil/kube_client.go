package testutil

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
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

func (f FakeClientFactory) GetConfigClient() (cfgClient.ClusterConfigInterface, error) {
	return nil, fmt.Errorf("NOT IMPLEMENTED")
}
