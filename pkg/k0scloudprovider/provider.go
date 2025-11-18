// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package k0scloudprovider

import (
	cloudprovider "k8s.io/cloud-provider"
)

const (
	Name = "k0s-cloud-provider"
)

type provider struct {
	instances cloudprovider.InstancesV2
}

var _ cloudprovider.Interface = (*provider)(nil)

// newProvider creates a new cloud provider using the provided
// `AddressCollector`
func newProvider(ac AddressCollector) *provider {
	return &provider{
		instances: newInstancesV2(ac),
	}
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping or run custom controllers specific to the cloud provider.
// Any tasks started here should be cleaned up when the stop channel closes.
func (p *provider) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	// Not supported
}

// LoadBalancer returns a balancer interface. Also returns true if the interface is supported, false otherwise.
func (p *provider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	// Not supported
	return nil, false
}

// Instances returns an instances interface. Also returns true if the interface is supported, false otherwise.
func (p *provider) Instances() (cloudprovider.Instances, bool) {
	// Not supported
	return nil, false
}

// InstancesV2 is an implementation for instances and should only be implemented by external cloud providers.
// Implementing InstancesV2 is behaviorally identical to Instances but is optimized to significantly reduce
// API calls to the cloud provider when registering and syncing nodes. Implementation of this interface will
// disable calls to the Zones interface. Also returns true if the interface is supported, false otherwise.
func (p *provider) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return p.instances, true
}

// Zones returns a zones interface. Also returns true if the interface is supported, false otherwise.
//
// Deprecated: Zones is deprecated in favor of retrieving zone/region information from InstancesV2.
// This interface will not be called if InstancesV2 is enabled.
func (p *provider) Zones() (cloudprovider.Zones, bool) {
	// Not supported
	return nil, false
}

// Clusters returns a clusters interface.  Also returns true if the interface is supported, false otherwise.
func (p *provider) Clusters() (cloudprovider.Clusters, bool) {
	// Not supported
	return nil, false
}

// Routes returns a routes interface along with whether the interface is supported.
func (p *provider) Routes() (cloudprovider.Routes, bool) {
	// Not supported
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (p *provider) ProviderName() string {
	return Name
}

// HasClusterID returns true if a ClusterID is required and set
func (p *provider) HasClusterID() bool {
	// Not supported
	return false
}
