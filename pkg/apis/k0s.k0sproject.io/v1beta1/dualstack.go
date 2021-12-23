package v1beta1

// DualStack defines network configuration for ipv4\ipv6 mixed cluster setup
type DualStack struct {
	Enabled         bool   `json:"enabled,omitempty"`
	IPv6PodCIDR     string `json:"IPv6podCIDR,omitempty"`
	IPv6ServiceCIDR string `json:"IPv6serviceCIDR,omitempty"`
}

// DefaultDualStack builds default values
func DefaultDualStack() DualStack {
	return DualStack{}
}
