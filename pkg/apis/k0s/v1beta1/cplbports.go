// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"cmp"
	"fmt"
	"math"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// LoadBalancedPorts selects the control plane ports that are load balanced on the virtual IPs.
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

// The default port where the userspace proxy listens for the API server port.
const defaultUserSpaceProxyPort = 6444

// BalancedPort is a control plane port that is load balanced on the virtual IPs.
// +k8s:deepcopy-gen=false
type BalancedPort struct {
	// The port on the virtual IPs and on the controllers.
	Port int
	// The port where the userspace proxy listens for it.
	ProxyPort int
}

// BalancedPorts returns the ports to load balance, or none if the load balancer is disabled.
// The userspace proxy listens on userSpaceProxyBindPort for the API server port, and on the next two for the others.
func (k *KeepalivedSpec) BalancedPorts(apiPort, konnectivityAgentPort, k0sAPIPort int) []BalancedPort {
	if k.DisableLoadBalancer {
		return nil
	}
	proxyPort := cmp.Or(k.UserSpaceProxyPort, defaultUserSpaceProxyPort)
	ports := k.LoadBalancedPorts
	var balanced []BalancedPort
	if ports.isAPIEnabled() {
		balanced = append(balanced, BalancedPort{Port: apiPort, ProxyPort: proxyPort})
	}
	if ports.isKonnectivityEnabled() {
		balanced = append(balanced, BalancedPort{Port: konnectivityAgentPort, ProxyPort: proxyPort + 1})
	}
	if ports.isK0sAPIEnabled() {
		balanced = append(balanced, BalancedPort{Port: k0sAPIPort, ProxyPort: proxyPort + 2})
	}
	return balanced
}

func (p *LoadBalancedPorts) isAPIEnabled() bool {
	return p == nil || p.API == nil || *p.API
}

func (p *LoadBalancedPorts) isKonnectivityEnabled() bool {
	return p != nil && p.Konnectivity != nil && *p.Konnectivity
}

func (p *LoadBalancedPorts) isK0sAPIEnabled() bool {
	return p != nil && p.K0sAPI != nil && *p.K0sAPI
}

func (k *KeepalivedSpec) validateLoadBalancedPorts(path *field.Path) (errs field.ErrorList) {
	// Only the proxy ports matter, so the controller ports are left out.
	ports := k.BalancedPorts(0, 0, 0)
	// Keepalived's virtual servers don't use the userspace proxy ports.
	if len(ports) == 0 || len(k.VirtualServers) > 0 {
		return nil
	}
	// The base port is checked with its own field, so only the extra ports matter here.
	if lastPort := ports[len(ports)-1].ProxyPort; lastPort != k.UserSpaceProxyPort && lastPort > math.MaxUint16 {
		errs = append(errs, field.Invalid(path.Child("userSpaceProxyBindPort"), k.UserSpaceProxyPort,
			fmt.Sprintf("loadBalancedPorts needs the ports %d to %d for the userspace proxy", k.UserSpaceProxyPort, lastPort)))
	}
	return errs
}
