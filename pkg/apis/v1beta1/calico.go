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

// Calico defines the calico related config options
type Calico struct {
	Mode                    string `yaml:"mode"`
	VxlanPort               int    `yaml:"vxlanPort"`
	VxlanVNI                int    `yaml:"vxlanVNI"`
	MTU                     int    `yaml:"mtu"`
	EnableWireguard         bool   `yaml:"wireguard"`
	FlexVolumeDriverPath    string `yaml:"flexVolumeDriverPath"`
	WithWindowsNodes        bool   `yaml:"withWindowsNodes"`
	Overlay                 string `yaml:"overlay" validate:"oneof=Always Never CrossSubnet"`
	IPAutodetectionMethod   string `yaml:"ipAutodetectionMethod,omitempty"`
	IPv6AutodetectionMethod string `yaml:"ipV6AutodetectionMethod,omitempty"`
}

// DefaultCalico returns sane defaults for calico
func DefaultCalico() *Calico {
	return &Calico{
		Mode:                    "vxlan",
		VxlanPort:               4789,
		VxlanVNI:                4096,
		MTU:                     0,
		EnableWireguard:         false,
		FlexVolumeDriverPath:    "/usr/libexec/k0s/kubelet-plugins/volume/exec/nodeagent~uds",
		WithWindowsNodes:        false,
		Overlay:                 "Always",
		IPAutodetectionMethod:   "",
		IPv6AutodetectionMethod: "",
	}
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (c *Calico) UnmarshalYAML(unmarshal func(interface{}) error) error {
	c.Mode = "vxlan"
	c.VxlanPort = 4789
	c.VxlanVNI = 4096
	c.MTU = 1450
	c.EnableWireguard = false
	c.WithWindowsNodes = false
	c.FlexVolumeDriverPath = "/usr/libexec/kubernetes/kubelet-plugins/volume/exec/nodeagent~uds"
	c.Overlay = "Always"
	c.IPAutodetectionMethod = ""
	c.IPv6AutodetectionMethod = ""

	type ycalico Calico
	yc := (*ycalico)(c)
	if err := unmarshal(yc); err != nil {
		return err
	}

	return nil
}
