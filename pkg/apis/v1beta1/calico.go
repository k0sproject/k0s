/*
Copyright 2020 Mirantis, Inc.

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
	Mode            string `yaml:"mode"`
	VxlanPort       int    `yaml:"vxlanPort"`
	VxlanVNI        int    `yaml:"vxlanVNI"`
	MTU             int    `yaml:"mtu"`
	EnableWireguard bool   `yaml:"wireguard"`
}

// DefaultCalico returns sane defaults for calico
func DefaultCalico() *Calico {
	return &Calico{
		Mode:            "vxlan",
		VxlanPort:       4789,
		VxlanVNI:        4096,
		MTU:             1450,
		EnableWireguard: false,
	}
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (c *Calico) UnmarshalYAML(unmarshal func(interface{}) error) error {
	c.Mode = "vxlan"
	c.VxlanPort = 4789
	c.VxlanVNI = 4096
	c.MTU = 1450
	c.EnableWireguard = false

	type ycalico Calico
	yc := (*ycalico)(c)
	if err := unmarshal(yc); err != nil {
		return err
	}

	return nil
}
