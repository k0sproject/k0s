// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// LoadBalancedPorts selects the control plane ports that the userspace proxy load balances on the virtual IPs.
type LoadBalancedPorts struct {
	// Load balance the API server port. Enabled by default.
	// +kubebuilder:default=true
	// +optional
	API *bool `json:"api,omitempty"`

	// Load balance the konnectivity agent port, so that agents connecting via a virtual IP reach every
	// konnectivity server. Disabled by default.
	// +optional
	Konnectivity *bool `json:"konnectivity,omitempty"`

	// Load balance the k0s API port, which joining controllers use. Disabled by default.
	// +optional
	K0sAPI *bool `json:"k0sApi,omitempty"`
}

// IsAPIEnabled reports whether the API server port is load balanced, which is the default.
func (p *LoadBalancedPorts) IsAPIEnabled() bool {
	return p == nil || p.API == nil || *p.API
}

// IsKonnectivityEnabled reports whether the konnectivity agent port is load balanced.
func (p *LoadBalancedPorts) IsKonnectivityEnabled() bool {
	return p != nil && p.Konnectivity != nil && *p.Konnectivity
}

// IsK0sAPIEnabled reports whether the k0s API port is load balanced.
func (p *LoadBalancedPorts) IsK0sAPIEnabled() bool {
	return p != nil && p.K0sAPI != nil && *p.K0sAPI
}

// UserSpaceProxyKonnectivityPort returns the port where the userspace proxy listens for konnectivity agents.
func (k *KeepalivedSpec) UserSpaceProxyKonnectivityPort() int {
	return k.UserSpaceProxyPort + 1
}

// UserSpaceProxyK0sAPIPort returns the port where the userspace proxy listens for the k0s API.
func (k *KeepalivedSpec) UserSpaceProxyK0sAPIPort() int {
	return k.UserSpaceProxyPort + 2
}

func (k *KeepalivedSpec) validateLoadBalancedPorts(path *field.Path) (errs field.ErrorList) {
	ports := k.LoadBalancedPorts
	others := ports.IsKonnectivityEnabled() || ports.IsK0sAPIEnabled()
	if len(k.VirtualServers) > 0 && (others || !ports.IsAPIEnabled()) {
		errs = append(errs, field.Forbidden(path.Child("loadBalancedPorts"), "is only supported by the userspace proxy, not with virtualServers"))
	}
	if others && k.UserSpaceProxyK0sAPIPort() > 65535 {
		errs = append(errs, field.Invalid(path.Child("userSpaceProxyBindPort"), k.UserSpaceProxyPort,
			fmt.Sprintf("loadBalancedPorts needs the ports %d to %d for the userspace proxy", k.UserSpaceProxyPort, k.UserSpaceProxyK0sAPIPort())))
	}
	return errs
}
