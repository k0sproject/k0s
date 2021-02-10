package v1beta1

import (
	"fmt"
	"net"
)

// DualStack defines network configuration for ipv4\ipv6 mixed cluster setup
type DualStack struct {
	Enabled         bool   `yaml:"enabled,omitempty"`
	IPv6PodCIDR     string `yaml:"IPv6podCIDR,omitempty"`
	IPv6ServiceCIDR string `yaml:"IPv6serviceCIDR,omitempty"`
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

// InternalAPIAddress calculates the internal API address of configured service CIDR block.
func (ds DualStack) InternalAPIAddress() (string, error) {
	if !ds.Enabled {
		return "", nil
	}
	_, ipnet, err := net.ParseCIDR(ds.IPv6ServiceCIDR)
	if err != nil {
		return "", fmt.Errorf("can't parse ds.IPv6ServiceCidr `%s`: %v", ds.IPv6ServiceCIDR, err)
	}

	address := ipnet.IP.To16()
	address[15] = address[15] + 1
	return address.String(), nil
}

// DefaultDualStack builds default values
func DefaultDualStack() DualStack {
	return DualStack{}
}
