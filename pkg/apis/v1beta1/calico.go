package v1beta1

// Calico defines the calico related config options
type Calico struct {
	Mode      string `yaml:"mode"`
	VxlanPort int    `yaml:"vxlanPort"`
	VxlanVNI  int    `yaml:"vxlanVNI"`
	MTU       int    `yaml:"mtu"`
}

// DefaultCalico returns sane defaults for calico
func DefaultCalico() *Calico {
	return &Calico{
		Mode:      "vxlan",
		VxlanPort: 4789,
		VxlanVNI:  4096,
		MTU:       1450,
	}
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (c *Calico) UnmarshalYAML(unmarshal func(interface{}) error) error {
	c.Mode = "vxlan"
	c.VxlanPort = 4789
	c.VxlanVNI = 4096
	c.MTU = 1450

	type ycalico Calico
	yc := (*ycalico)(c)
	if err := unmarshal(yc); err != nil {
		return err
	}

	return nil
}
