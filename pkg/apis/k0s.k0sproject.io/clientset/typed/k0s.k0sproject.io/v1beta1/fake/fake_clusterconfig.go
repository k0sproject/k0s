/*
Copyright k0s authors

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeClusterConfigs implements ClusterConfigInterface
type FakeClusterConfigs struct {
	Fake *FakeK0sV1beta1
	ns   string
}

var clusterconfigsResource = schema.GroupVersionResource{Group: "k0s.k0sproject.io", Version: "v1beta1", Resource: "clusterconfigs"}

var clusterconfigsKind = schema.GroupVersionKind{Group: "k0s.k0sproject.io", Version: "v1beta1", Kind: "ClusterConfig"}

// Get takes name of the clusterConfig, and returns the corresponding clusterConfig object, and an error if there is any.
func (c *FakeClusterConfigs) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1beta1.ClusterConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(clusterconfigsResource, c.ns, name), &v1beta1.ClusterConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ClusterConfig), err
}

// List takes label and field selectors, and returns the list of ClusterConfigs that match those selectors.
func (c *FakeClusterConfigs) List(ctx context.Context, opts v1.ListOptions) (result *v1beta1.ClusterConfigList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(clusterconfigsResource, clusterconfigsKind, c.ns, opts), &v1beta1.ClusterConfigList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1beta1.ClusterConfigList{ListMeta: obj.(*v1beta1.ClusterConfigList).ListMeta}
	for _, item := range obj.(*v1beta1.ClusterConfigList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterConfigs.
func (c *FakeClusterConfigs) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(clusterconfigsResource, c.ns, opts))

}

// Create takes the representation of a clusterConfig and creates it.  Returns the server's representation of the clusterConfig, and an error, if there is any.
func (c *FakeClusterConfigs) Create(ctx context.Context, clusterConfig *v1beta1.ClusterConfig, opts v1.CreateOptions) (result *v1beta1.ClusterConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(clusterconfigsResource, c.ns, clusterConfig), &v1beta1.ClusterConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ClusterConfig), err
}

// Update takes the representation of a clusterConfig and updates it. Returns the server's representation of the clusterConfig, and an error, if there is any.
func (c *FakeClusterConfigs) Update(ctx context.Context, clusterConfig *v1beta1.ClusterConfig, opts v1.UpdateOptions) (result *v1beta1.ClusterConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(clusterconfigsResource, c.ns, clusterConfig), &v1beta1.ClusterConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ClusterConfig), err
}

// Delete takes name of the clusterConfig and deletes it. Returns an error if one occurs.
func (c *FakeClusterConfigs) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(clusterconfigsResource, c.ns, name, opts), &v1beta1.ClusterConfig{})

	return err
}
