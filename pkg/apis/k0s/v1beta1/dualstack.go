// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

// DualStack defines network configuration for ipv4\ipv6 mixed cluster setup
type DualStack struct {
	// +kubebuilder:default=false
	// +optional
	Enabled         bool   `json:"enabled"`
	IPv6PodCIDR     string `json:"IPv6podCIDR,omitempty"`
	IPv6ServiceCIDR string `json:"IPv6serviceCIDR,omitempty"`
}

// DefaultDualStack builds default values
func DefaultDualStack() DualStack {
	return DualStack{}
}
