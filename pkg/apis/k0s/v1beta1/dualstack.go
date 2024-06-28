/*
Copyright 2021 k0s authors

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
