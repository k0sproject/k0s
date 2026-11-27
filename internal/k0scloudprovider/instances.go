// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package k0scloudprovider

import (
	"context"

	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
)

// cloudprovider.InstancesV2

type instancesV2 struct {
	addressCollector AddressCollector
}

var _ cloudprovider.InstancesV2 = (*instancesV2)(nil)

// newInstancesV2 creates a new `cloudprovider.InstancesV2` using the
// provided `AddressCollector`
func newInstancesV2(ac AddressCollector) cloudprovider.InstancesV2 {
	return &instancesV2{addressCollector: ac}
}

// InstanceExists returns true if the instance for the given node exists according to the cloud provider.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (i *instancesV2) InstanceExists(ctx context.Context, node *v1.Node) (bool, error) {
	return true, nil
}

// InstanceShutdown returns true if the instance is shutdown according to the cloud provider.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (i *instancesV2) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
	return false, nil
}

// InstanceMetadata returns the instance's metadata. The values returned in InstanceMetadata are
// translated into specific fields and labels in the Node object on registration.
// Implementations should always check node.spec.providerID first when trying to discover the instance
// for a given node. In cases where node.spec.providerID is empty, implementations can use other
// properties of the node like its name, labels and annotations.
func (i *instancesV2) InstanceMetadata(ctx context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	return &cloudprovider.InstanceMetadata{
		ProviderID:    Name + "://" + node.Name,
		InstanceType:  Name,
		NodeAddresses: i.addressCollector(node),
	}, nil
}
