// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import "k8s.io/utils/ptr"

// KubeRouter defines the kube-router related config options
type KubeRouter struct {
	// Auto-detection of used MTU (default: true)
	// +kubebuilder:default=true
	AutoMTU *bool `json:"autoMTU,omitempty"`

	// Override MTU setting (autoMTU must be set to false)
	MTU int `json:"mtu,omitempty"`

	// Kube-router metrics server port. Set to 0 to disable metrics  (default: 8080)
	MetricsPort int `json:"metricsPort,omitempty"`

	// Admits three values: "Enabled" enables it globally, "Allowed" allows but services must be annotated explicitly and "Disabled"
	// Defaults to "Enabled"
	// +kubebuilder:default=Enabled
	Hairpin Hairpin `json:"hairpin,omitempty"`

	// Deprecated: Use hairpin instead. Activates Hairpin Mode (allow a Pod behind a Service to communicate to its own ClusterIP:Port)
	//+kubebuilder:deprecatedversion:warning="Use hairpin instead. Activates Hairpin Mode (allow a Pod behind a Service to communicate to its own ClusterIP:Port)"
	HairpinMode bool `json:"hairpinMode,omitempty"`

	// IP masquerade for traffic originating from the pod network, and destined outside of it (default: false)
	IPMasq bool `json:"ipMasq,omitempty"`

	// Comma-separated list of global peer addresses
	//
	// Deprecated: Use extraArgs with peerRouterASNs instead
	//+kubebuilder:deprecatedversion:warning="Use extraArgs with peerRouterASNs instead"
	PeerRouterASNs string `json:"peerRouterASNs,omitempty"`

	// Comma-separated list of global peer ASNs
	//
	// Deprecated: Use extraArgs with peerRouterIPs instead
	//+kubebuilder:deprecatedversion:warning="Use extraArgs with peerRouterIPs instead"
	PeerRouterIPs string `json:"peerRouterIPs,omitempty"`

	// ExtraArgs are extra arguments to pass to kube-router
	// Can be also used to override the default k0s managed kube-router arguments
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`

	// Slice of strings with raw arguments to pass to the kube-router process
	// These arguments will be appended to the `ExtraArgs` and aren't validated at all.
	// ExtraArgs are recommended over RawArgs. If possible use ExtraArgs to set arguments.
	RawArgs []string `json:"rawArgs,omitempty"`
}

func (k *KubeRouter) IsAutoMTU() bool {
	return k == nil || k.AutoMTU == nil || *k.AutoMTU
}

// +kubebuilder:validation:Enum=Enabled;Allowed;Disabled
type Hairpin string

const (
	HairpinEnabled  Hairpin = "Enabled"
	HairpinAllowed  Hairpin = "Allowed"
	HairpinDisabled Hairpin = "Disabled"
	// Necessary for backwards compatibility with HairpinMode
	HairpinUndefined Hairpin = ""
)

// DefaultKubeRouter returns the default config for kube-router
func DefaultKubeRouter() *KubeRouter {
	return &KubeRouter{
		MTU:         0,
		AutoMTU:     ptr.To(true),
		MetricsPort: 8080,
		Hairpin:     HairpinEnabled,
	}
}
