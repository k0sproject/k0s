/*
Copyright 2023 k0s authors

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
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakeClusterConfigLists implements ClusterConfigListInterface
type FakeClusterConfigLists struct {
	Fake *FakeK0sV1beta1
	ns   string
}

var clusterconfiglistsResource = schema.GroupVersionResource{Group: "k0s.k0sproject.io", Version: "v1beta1", Resource: "clusterconfiglists"}

var clusterconfiglistsKind = schema.GroupVersionKind{Group: "k0s.k0sproject.io", Version: "v1beta1", Kind: "ClusterConfigList"}

// Create takes the representation of a clusterConfigList and creates it.  Returns the server's representation of the clusterConfigList, and an error, if there is any.
func (c *FakeClusterConfigLists) Create(ctx context.Context, clusterConfigList *v1beta1.ClusterConfigList, opts v1.CreateOptions) (result *v1beta1.ClusterConfigList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(clusterconfiglistsResource, c.ns, clusterConfigList), &v1beta1.ClusterConfigList{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ClusterConfigList), err
}
