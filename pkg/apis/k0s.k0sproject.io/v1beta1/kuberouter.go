/*
Copyright 2020 k0s authors

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

package v1beta1

// KubeRouter defines the kube-router related config options
type KubeRouter struct {
	// Auto-detection of used MTU (default: true)
	AutoMTU bool `json:"autoMTU"`
	// Override MTU setting (autoMTU must be set to false)
	MTU int `json:"mtu"`
	// Kube-router metrics server port. Set to 0 to disable metrics  (default: 8080)
	MetricsPort int `json:"metricsPort"`
	// Admits three values: "Enabled" enables it globally, "Allowed" allows but services must be annotated explicitly and "Disabled"
	// Defaults to "Enabled"
	// +kubebuilder:default=Enabled
	Hairpin Hairpin `json:"hairpin"`
	// DEPRECATED: Use hairpin instead. Activates Hairpin Mode (allow a Pod behind a Service to communicate to its own ClusterIP:Port)
	HairpinMode bool `json:"hairpinMode,omitempty"`
	// IP masquerade for traffic originating from the pod network, and destined outside of it (default: false)
	IPMasq bool `json:"ipMasq"`
	// Comma-separated list of global peer addresses
	PeerRouterASNs string `json:"peerRouterASNs"`
	// Comma-separated list of global peer ASNs
	PeerRouterIPs string `json:"peerRouterIPs"`
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
		AutoMTU:     true,
		MetricsPort: 8080,
		Hairpin:     HairpinEnabled,
	}
}
