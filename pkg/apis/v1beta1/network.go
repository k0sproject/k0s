package v1beta1

import (
	"net"

	"github.com/pkg/errors"
)

type Network struct {
	PodCIDR     string `yaml:"podCIDR"`
	ServiceCIDR string `yaml:"serviceCIDR"`
}

func DefaultNetwork() *Network {
	return &Network{
		PodCIDR:     "10.244.0.0/16",
		ServiceCIDR: "10.96.0.0/12",
	}
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
