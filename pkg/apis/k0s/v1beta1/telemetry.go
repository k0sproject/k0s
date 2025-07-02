// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import "k8s.io/utils/ptr"

var _ Validateable = (*ClusterTelemetry)(nil)

// ClusterTelemetry holds telemetry related settings
type ClusterTelemetry struct {
	// +kubebuilder:default=false
	Enabled *bool `json:"enabled,omitempty"`
}

func (t *ClusterTelemetry) IsEnabled() bool {
	return t != nil && t.Enabled != nil && *t.Enabled
}

// DefaultClusterTelemetry default settings
func DefaultClusterTelemetry() *ClusterTelemetry {
	return &ClusterTelemetry{
		Enabled: ptr.To(false),
	}
}

// Validate stub for Validateable interface
func (c *ClusterTelemetry) Validate() []error {
	return nil
}
