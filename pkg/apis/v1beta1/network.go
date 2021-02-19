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

import (
	"fmt"
	"net"

	"github.com/pkg/errors"
	utilnet "k8s.io/utils/net"
)

// Network defines the network related config options
type Network struct {
	PodCIDR     string    `yaml:"podCIDR"`
	ServiceCIDR string    `yaml:"serviceCIDR"`
	Provider    string    `yaml:"provider"`
	Calico      *Calico   `yaml:"calico"`
	DualStack   DualStack `yaml:"dualStack,omitempty"`
}

// DefaultNetwork creates the Network config struct with sane default values
func DefaultNetwork() *Network {
	return &Network{
		PodCIDR:     "10.244.0.0/16",
		ServiceCIDR: "10.96.0.0/12",
		Provider:    "calico",
		Calico:      DefaultCalico(),
		DualStack:   DefaultDualStack(),
	}
}

// Validate validates all the settings make sense and should work
func (n *Network) Validate() []error {
	var errors []error
	if n.Provider != "calico" && n.Provider != "custom" {
		errors = append(errors, fmt.Errorf("unsupported network provider: %s", n.Provider))
	}
	if n.Provider == "calico" && n.DualStack.Enabled && n.Calico.Mode != "bird" {
		errors = append(errors, fmt.Errorf("network dual stack is supported only for calico mode `bird`"))
	}

	_, _, err := net.ParseCIDR(n.PodCIDR)
	if err != nil {
		errors = append(errors, fmt.Errorf("invalid pod CIDR %s", n.PodCIDR))
	}

	_, _, err = net.ParseCIDR(n.ServiceCIDR)
	if err != nil {
		errors = append(errors, fmt.Errorf("invalid service CIDR %s", n.ServiceCIDR))
	}

	if n.DualStack.Enabled {
		_, _, err := net.ParseCIDR(n.DualStack.IPv6PodCIDR)
		if err != nil {
			errors = append(errors, fmt.Errorf("invalid pod IPv6 CIDR %s", n.DualStack.IPv6PodCIDR))
		}
		_, _, err = net.ParseCIDR(n.DualStack.IPv6ServiceCIDR)
		if err != nil {
			errors = append(errors, fmt.Errorf("invalid service IPv6 CIDR %s", n.DualStack.IPv6ServiceCIDR))
		}
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

// InternalAPIAddresses calculates the internal API address of configured service CIDR block.
func (n *Network) InternalAPIAddresses() ([]string, error) {
	cidrs := []string{n.ServiceCIDR}

	if n.DualStack.Enabled {
		cidrs = append(cidrs, n.DualStack.IPv6ServiceCIDR)
	}

	parsedCIDRs, err := utilnet.ParseCIDRs(cidrs)
	if err != nil {
		return nil, fmt.Errorf("can't parse service cidr to build internal API address: %v", err)
	}

	stringifiedAddresses := make([]string, len(parsedCIDRs))
	for i, ip := range parsedCIDRs {
		apiIP, err := utilnet.GetIndexedIP(ip, 1)
		if err != nil {
			return nil, fmt.Errorf("can't build internal API address: %v", err)
		}
		stringifiedAddresses[i] = apiIP.String()
	}
	return stringifiedAddresses, nil
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (n *Network) UnmarshalYAML(unmarshal func(interface{}) error) error {
	n.Provider = "calico"

	type ynetwork Network
	yc := (*ynetwork)(n)

	if err := unmarshal(yc); err != nil {
		return err
	}

	if n.Provider == "calico" && n.Calico == nil {
		n.Calico = DefaultCalico()
	}

	return nil
}

// BuildServiceCIDR returns actual argument value for service cidr
func (n *Network) BuildServiceCIDR() string {
	if n.DualStack.Enabled {
		return n.DualStack.IPv6ServiceCIDR + "," + n.ServiceCIDR
	}
	return n.ServiceCIDR
}

// BuildPodCIDR returns actual argument value for pod cidr
func (n *Network) BuildPodCIDR() string {
	if n.DualStack.Enabled {
		return n.DualStack.IPv6PodCIDR + "," + n.PodCIDR
	}
	return n.PodCIDR
}
