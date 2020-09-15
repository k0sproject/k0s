package v1beta1

import (
	"fmt"
	"net"

	"github.com/pkg/errors"
)

// Network defines the network related config options
type Network struct {
	PodCIDR     string `yaml:"podCIDR"`
	ServiceCIDR string `yaml:"serviceCIDR"`
	Provider    string `yaml:"provider"`
}

// DefaultNetwork creates the Network config stcut with sane default values
func DefaultNetwork() *Network {
	return &Network{
		PodCIDR:     "10.244.0.0/16",
		ServiceCIDR: "10.96.0.0/12",
		Provider:    "calico",
	}
}

// Validate validates all the settings make sense and should work
func (n *Network) Validate() []error {
	var errors []error
	if n.Provider != "calico" && n.Provider != "custom" {
		errors = append(errors, fmt.Errorf("unsupported network provider: %s", n.Provider))
	}
	return errors
}

// DNSAddress calculates the 10th address of configured service CIDR block.
func (n *Network) DNSAddress() (string, error) {
	_, ipnet, err := net.ParseCIDR(n.ServiceCIDR)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse service CIDR %s: %s", n.ServiceCIDR, err.Error())
	}

	address := ipnet.IP.To4()
	prefixlen, _ := ipnet.Mask.Size()
	if prefixlen < 29 {
		address[3] = address[3] + 10
	} else {
		address[3] = address[3] + 2
	}

	if !ipnet.Contains(address) {
		return "", errors.Wrapf(err, "failed to calculate a valid DNS address: %s", address.String())
	}

	return address.String(), nil
}

// InternalAPIAddress calculates the internal API address of configured service CIDR block.
func (n *Network) InternalAPIAddress() (string, error) {
	_, ipnet, err := net.ParseCIDR(n.ServiceCIDR)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse service CIDR %s: %s", n.ServiceCIDR, err.Error())
	}

	address := ipnet.IP.To4()
	address[3] = address[3] + 1
	return address.String(), nil
}
