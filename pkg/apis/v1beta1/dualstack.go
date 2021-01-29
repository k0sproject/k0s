package v1beta1

// DualStack defines network configuration for ipv4\ipv6 mixed cluster setup
type DualStack struct {
	Enabled         bool   `yaml:"enabled"`
	IPv6PodCIDR     string `yaml:"IPv6podCIDR"`
	IPv6ServiceCIDR string `yaml:"IPv6serviceCIDR"`
}

// EnableDualStackFeatureGate adds ipv6 feature gate to the given args colllection
func (ds DualStack) EnableDualStackFeatureGate(args map[string]string) {
	if !ds.Enabled {
		return
	}
	fg, found := args["feature-gates"]
	if !found {
		args["feature-gates"] = "IPv6DualStack=true"
	} else {
		fg = fg + ",IPv6DualStack=true"
		args["feature-gates"] = fg
	}
}
