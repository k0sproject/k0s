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

	v1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeUpdateConfigs implements UpdateConfigInterface
type FakeUpdateConfigs struct {
	Fake *FakeAutopilotV1beta2
}

var updateconfigsResource = v1beta2.SchemeGroupVersion.WithResource("updateconfigs")

var updateconfigsKind = v1beta2.SchemeGroupVersion.WithKind("UpdateConfig")

// Get takes name of the updateConfig, and returns the corresponding updateConfig object, and an error if there is any.
func (c *FakeUpdateConfigs) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1beta2.UpdateConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(updateconfigsResource, name), &v1beta2.UpdateConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta2.UpdateConfig), err
}

// List takes label and field selectors, and returns the list of UpdateConfigs that match those selectors.
func (c *FakeUpdateConfigs) List(ctx context.Context, opts v1.ListOptions) (result *v1beta2.UpdateConfigList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(updateconfigsResource, updateconfigsKind, opts), &v1beta2.UpdateConfigList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1beta2.UpdateConfigList{ListMeta: obj.(*v1beta2.UpdateConfigList).ListMeta}
	for _, item := range obj.(*v1beta2.UpdateConfigList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested updateConfigs.
func (c *FakeUpdateConfigs) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(updateconfigsResource, opts))
}

// Create takes the representation of a updateConfig and creates it.  Returns the server's representation of the updateConfig, and an error, if there is any.
func (c *FakeUpdateConfigs) Create(ctx context.Context, updateConfig *v1beta2.UpdateConfig, opts v1.CreateOptions) (result *v1beta2.UpdateConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(updateconfigsResource, updateConfig), &v1beta2.UpdateConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta2.UpdateConfig), err
}

// Update takes the representation of a updateConfig and updates it. Returns the server's representation of the updateConfig, and an error, if there is any.
func (c *FakeUpdateConfigs) Update(ctx context.Context, updateConfig *v1beta2.UpdateConfig, opts v1.UpdateOptions) (result *v1beta2.UpdateConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(updateconfigsResource, updateConfig), &v1beta2.UpdateConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta2.UpdateConfig), err
}

// Delete takes name of the updateConfig and deletes it. Returns an error if one occurs.
func (c *FakeUpdateConfigs) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(updateconfigsResource, name, opts), &v1beta2.UpdateConfig{})
	return err
}
