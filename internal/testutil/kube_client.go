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
	"reflect"
	"strings"

	"github.com/k0sproject/k0s/internal/testutil/fakeclient"
	k0sclientset "github.com/k0sproject/k0s/pkg/client/clientset"
	k0sfake "github.com/k0sproject/k0s/pkg/client/clientset/fake"
	k0sscheme "github.com/k0sproject/k0s/pkg/client/clientset/scheme"
	etcdv1beta1 "github.com/k0sproject/k0s/pkg/client/clientset/typed/etcd/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/client/clientset/typed/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

var _ kubeutil.ClientFactoryInterface = (*FakeClientFactory)(nil)

// NewFakeClientFactory creates new client factory which uses internally only the kube fake client interface
func NewFakeClientFactory(objects ...runtime.Object) *FakeClientFactory {
	// Create a scheme containing all the kinds and types that k0s knows about.
	scheme := runtime.NewScheme()
	utilruntime.Must(kubernetesscheme.AddToScheme(scheme))
	utilruntime.Must(k0sscheme.AddToScheme(scheme))

	// Create a dynamic fake client that can deal with all that.
	fakeDynamic := dynamicfake.NewSimpleDynamicClient(scheme, objects...)
	fakeDynamic.Resources = makeAPIResourceLists(scheme)

	// Create a fake discovery client backed by the dynamic fake client.
	fakeDiscovery := &discoveryfake.FakeDiscovery{Fake: &fakeDynamic.Fake}

	// In order for the typed clients to share the same view of the world as the
	// dynamic client, they all must share the same (wrapped) object tracker. We
	// create typed clients using the dynamic client's object tracker and
	// transform between typed and unstructured objects.
	tracker := fakeclient.TypedObjectTrackerFrom(scheme, fakeDynamic)
	kubeClients := fakeclient.NewClientset[kubernetesfake.Clientset](fakeDiscovery, tracker)
	k0sClients := fakeclient.NewClientset[k0sfake.Clientset](fakeDiscovery, tracker)

	return &FakeClientFactory{
		DynamicClient:   fakeDynamic,
		Client:          kubeClients,
		DiscoveryClient: memory.NewMemCacheClient(fakeDiscovery),
		K0sClient:       k0sClients,
	}
}

type FakeClientFactory struct {
	DynamicClient   *dynamicfake.FakeDynamicClient
	Client          kubernetes.Interface
	DiscoveryClient discovery.CachedDiscoveryInterface
	K0sClient       k0sclientset.Interface
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

func (f *FakeClientFactory) GetK0sClient() (k0sclientset.Interface, error) {
	return f.K0sClient, nil
}

// Deprecated: Use [FakeClientFactory.GetK0sClient] instead.
func (f *FakeClientFactory) GetConfigClient() (k0sv1beta1.ClusterConfigInterface, error) {
	return f.K0sClient.K0sV1beta1().ClusterConfigs(constant.ClusterConfigNamespace), nil
}

func (f FakeClientFactory) GetRESTConfig() *rest.Config {
	return &rest.Config{}
}

// Deprecated: Use [FakeClientFactory.GetK0sClient] instead.
func (f FakeClientFactory) GetEtcdMemberClient() (etcdv1beta1.EtcdMemberInterface, error) {
	return f.K0sClient.EtcdV1beta1().EtcdMembers(), nil
}

// Extracts all kinds from scheme and builds API resource lists for fake discovery clients.
func makeAPIResourceLists(scheme *runtime.Scheme) (allResources []*metav1.APIResourceList) {
	// Create the array of API resource lists. Ensure that the preferred version
	// of any given group comes first. This is important for the fake discovery
	// client, as it will bluntly pick the first version as the preferred one.
	for _, gv := range scheme.PrioritizedVersionsAllGroups() {
		var resources []metav1.APIResource
		for kind, ty := range scheme.KnownTypes(gv) {
			// Skip list kinds themselves.
			if o, ok := reflect.New(ty).Interface().(runtime.Object); ok && meta.IsListType(o) {
				continue
			}

			// Skip kinds that don't have an associated list kind.
			if !scheme.Recognizes(gv.WithKind(kind + "List")) {
				continue
			}

			plural, singular := meta.UnsafeGuessKindToResource(gv.WithKind(kind))
			resource := metav1.APIResource{
				Name:         plural.Resource,
				SingularName: singular.Resource,
				Kind:         kind,
				Verbs:        metav1.Verbs{"get", "list", "watch", "create", "update", "patch", "delete"},
			}

			// Some duct tape for guessing cluster resources.
			// FIXME: Is there any way to figure this out reliably? We could scan
			// the clientsets if the factory methods have a string argument for
			// the namespace or not.
			switch {
			case strings.Contains(kind, "Node"),
				strings.Contains(kind, "Namespace"),
				strings.Contains(kind, "Cluster"):
				resource.Namespaced = false
			default:
				resource.Namespaced = true
			}

			resources = append(resources, resource)
		}

		allResources = append(allResources, &metav1.APIResourceList{
			GroupVersion: gv.String(),
			APIResources: resources,
		})
	}

	return
}
