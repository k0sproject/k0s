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

import (
	"encoding/json"
	"slices"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Calico defines the calico related config options
type Calico struct {
	// Enable wireguard-based encryption (default: false)
	EnableWireguard bool `json:"wireguard,omitempty"`

	// Environment variables to configure Calico node (see https://docs.projectcalico.org/reference/node/configuration)
	EnvVars map[string]string `json:"envVars,omitempty"`

	// The host path for Calicos flex-volume-driver(default: /usr/libexec/k0s/kubelet-plugins/volume/exec/nodeagent~uds)
	// +kubebuilder:default="/usr/libexec/k0s/kubelet-plugins/volume/exec/nodeagent~uds"
	FlexVolumeDriverPath string `json:"flexVolumeDriverPath,omitempty"`

	// Host's IP Auto-detection method for Calico (see https://docs.projectcalico.org/reference/node/configuration#ip-autodetection-methods)
	IPAutodetectionMethod string `json:"ipAutodetectionMethod,omitempty"`

	// Host's IPv6 Auto-detection method for Calico
	IPv6AutodetectionMethod string `json:"ipV6AutodetectionMethod,omitempty"`

	// MTU for overlay network (default: 1450)
	// +kubebuilder:default=1450
	MTU int `json:"mtu,omitempty"`

	// +kubebuilder:default=vxlan
	Mode CalicoMode `json:"mode,omitempty"`

	// Overlay Type (Always, Never or CrossSubnet).
	// Will be ignored in vxlan mode.
	// +kubebuilder:default=Always
	Overlay string `json:"overlay,omitempty"`

	// The UDP port for VXLAN (default: 4789)
	// +kubebuilder:default=4789
	VxlanPort int `json:"vxlanPort,omitempty"`

	// The virtual network ID for VXLAN (default: 4096)
	// +kubebuilder:default=4096
	VxlanVNI int `json:"vxlanVNI,omitempty"`
}

// Indicates the Calico backend to use. Either `bird` or `vxlan`.
// The deprecated legacy value `ipip` is also accepted.
// +kubebuilder:validation:Enum=bird;vxlan;ipip
type CalicoMode string

const (
	CalicoModeBIRD  CalicoMode = "bird"
	CalicoModeVXLAN CalicoMode = "vxlan"
	CalicoModeIPIP  CalicoMode = "ipip" // Deprecated: Use [CalicoModeBIRD] instead.
)

// DefaultCalico returns sane defaults for calico
func DefaultCalico() *Calico {
	return &Calico{
		Mode:                 CalicoModeVXLAN,
		VxlanPort:            4789,
		VxlanVNI:             4096,
		MTU:                  1450,
		FlexVolumeDriverPath: "/usr/libexec/k0s/kubelet-plugins/volume/exec/nodeagent~uds",
		Overlay:              "Always",
	}
}

// UnmarshalJSON sets in some sane defaults when unmarshaling the data from JSON
func (c *Calico) UnmarshalJSON(data []byte) error {
	c.Mode = CalicoModeVXLAN
	c.VxlanPort = 4789
	c.VxlanVNI = 4096
	c.MTU = 1450
	c.FlexVolumeDriverPath = "/usr/libexec/kubernetes/kubelet-plugins/volume/exec/nodeagent~uds"
	c.Overlay = "Always"

	type calico Calico
	jc := (*calico)(c)
	return json.Unmarshal(data, jc)
}

func (c *Calico) Validate(path *field.Path) (errs field.ErrorList) {
	if c == nil {
		return
	}

	if c.Mode == "" {
		errs = append(errs, field.Required(path.Child("mode"), ""))
	} else if allowed := []CalicoMode{
		CalicoModeBIRD, CalicoModeVXLAN, CalicoModeIPIP,
	}; !slices.Contains(allowed, c.Mode) {
		errs = append(errs, field.NotSupported(path.Child("mode"), c.Mode, allowed))
	}

	return
}
