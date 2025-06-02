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
	"errors"
	"fmt"
	"net"
	"slices"

	"k8s.io/apimachinery/pkg/util/validation/field"
	utilnet "k8s.io/utils/net"

	"github.com/asaskevich/govalidator"
)

var _ Validateable = (*Network)(nil)

// Network defines the network related config options
type Network struct {
	Calico    *Calico   `json:"calico,omitempty"`
	DualStack DualStack `json:"dualStack,omitempty"`

	KubeProxy  *KubeProxy  `json:"kubeProxy,omitempty"`
	KubeRouter *KubeRouter `json:"kuberouter,omitempty"`

	// NodeLocalLoadBalancing defines the configuration options related to k0s's
	// node-local load balancing feature.
	// NOTE: This feature is currently unsupported on ARMv7!
	NodeLocalLoadBalancing *NodeLocalLoadBalancing `json:"nodeLocalLoadBalancing,omitempty"`

	// ControlPlaneLoadBalancing defines the configuration options related to k0s's
	// control plane load balancing feature.
	ControlPlaneLoadBalancing *ControlPlaneLoadBalancingSpec `json:"controlPlaneLoadBalancing,omitempty"`

	// Pod network CIDR to use in the cluster
	// +kubebuilder:default="10.244.0.0/16"
	PodCIDR string `json:"podCIDR,omitempty"`
	// Network provider (valid values: calico, kuberouter, or custom)
	// +kubebuilder:validation:Enum=kuberouter;calico;custom
	// +kubebuilder:default=kuberouter
	Provider string `json:"provider,omitempty"`
	// Network CIDR to use for cluster VIP services
	// +kubebuilder:default="10.96.0.0/12"
	ServiceCIDR string `json:"serviceCIDR,omitempty"`
	// Cluster Domain
	// +kubebuilder:default="cluster.local"
	ClusterDomain string `json:"clusterDomain,omitempty"`

	// PrimaryAddressFamily defines the primary family for the cluster.
	// If empty, k0s determines it based on `.spec.API.ExternalAddress`,
	// if this isn't present it will use `.spec.API.Address.`.
	// If both addresses are empty or the chosen address is a hostname, defaults to `IPv4`.
	// +Kubebuilder:validation:Enum=IPv4;IPv6
	PrimaryAddressFamily PrimaryAddressFamilyType `json:"primaryAddressFamily,omitempty"`
}

type PrimaryAddressFamilyType string

const (
	PrimaryFamilyUnknown PrimaryAddressFamilyType = ""
	PrimaryFamilyIPv4    PrimaryAddressFamilyType = "IPv4"
	PrimaryFamilyIPv6    PrimaryAddressFamilyType = "IPv6"
)

// DefaultNetwork creates the Network config struct with sane default values
func DefaultNetwork() *Network {
	return &Network{
		PodCIDR:                "10.244.0.0/16",
		ServiceCIDR:            "10.96.0.0/12",
		Provider:               "kuberouter",
		KubeRouter:             DefaultKubeRouter(),
		DualStack:              DefaultDualStack(),
		KubeProxy:              DefaultKubeProxy(),
		NodeLocalLoadBalancing: DefaultNodeLocalLoadBalancing(),
		ClusterDomain:          "cluster.local",
	}
}

// Validate validates all the settings make sense and should work
func (n *Network) Validate() []error {
	if n == nil {
		return nil
	}

	var errors []error

	if n.Provider == "" {
		errors = append(errors, field.Required(field.NewPath("provider"), ""))
	} else if n.Provider != "calico" && n.Provider != "custom" && n.Provider != "kuberouter" {
		errors = append(errors, field.NotSupported(field.NewPath("provider"), n.Provider, []string{"kuberouter", "calico", "custom"}))
	}

	_, _, err := net.ParseCIDR(n.PodCIDR)
	if err != nil {
		errors = append(errors, field.Invalid(field.NewPath("podCIDR"), n.PodCIDR, "invalid CIDR address"))
	}

	_, _, err = net.ParseCIDR(n.ServiceCIDR)
	if err != nil {
		errors = append(errors, field.Invalid(field.NewPath("serviceCIDR"), n.ServiceCIDR, "invalid CIDR address"))
	}

	if !govalidator.IsDNSName(n.ClusterDomain) {
		errors = append(errors, field.Invalid(field.NewPath("clusterDomain"), n.ClusterDomain, "invalid DNS name"))
	}

	if n.DualStack.Enabled {
		if n.Provider == "calico" && n.Calico.Mode != CalicoModeBIRD {
			errors = append(errors, field.Forbidden(field.NewPath("calico", "mode"), fmt.Sprintf("dual-stack for calico is only supported for mode `%s`", CalicoModeBIRD)))
		}
		_, _, err := net.ParseCIDR(n.DualStack.IPv6PodCIDR)
		if err != nil {
			errors = append(errors, field.Invalid(field.NewPath("dualStack", "IPv6podCIDR"), n.DualStack.IPv6PodCIDR, "invalid CIDR address"))
		}
		_, _, err = net.ParseCIDR(n.DualStack.IPv6ServiceCIDR)
		if err != nil {
			errors = append(errors, field.Invalid(field.NewPath("dualStack", "IPv6serviceCIDR"), n.DualStack.IPv6ServiceCIDR, "invalid CIDR address"))
		}
	}

	if n.PrimaryAddressFamily != "" {
		if allowed := []PrimaryAddressFamilyType{PrimaryFamilyIPv4, PrimaryFamilyIPv6}; !slices.Contains(allowed, n.PrimaryAddressFamily) {
			err := field.NotSupported(field.NewPath("addressFamily"), n.PrimaryAddressFamily, allowed)
			errors = append(errors, err)
		}
	}

	errors = append(errors, n.KubeProxy.Validate()...)
	for _, err := range n.Calico.Validate(field.NewPath("calico")) {
		errors = append(errors, err)
	}
	for _, err := range n.NodeLocalLoadBalancing.Validate(field.NewPath("nodeLocalLoadBalancing")) {
		errors = append(errors, err)
	}

	return errors
}

// DNSAddress calculates the 10th address of configured service CIDR block.
func (n *Network) DNSAddress() (string, error) {
	_, ipnet, err := net.ParseCIDR(n.ServiceCIDR)
	if err != nil {
		return "", fmt.Errorf("failed to parse service CIDR %q: %w", n.ServiceCIDR, err)
	}

	address := slices.Clone(ipnet.IP.To4())
	if address == nil {
		// The network address is not an IPv4 address. This can only happen if
		// k0s is running in IPv6-only mode, which is currently not a supported
		// configuration. In dual-stack mode, the IPv6 CIDR is stored in
		// n.DualStack.IPv6ServiceCIDR. Error out until it is clear how to
		// properly calculate the DNS address for a v6 network.
		return "", fmt.Errorf("%w: DNS address calculation for non-v4 CIDR: %s", errors.ErrUnsupported, n.ServiceCIDR)
	}

	prefixlen, _ := ipnet.Mask.Size()
	if prefixlen < 29 {
		address[3] += 10
	} else {
		address[3] += 2
	}

	if !ipnet.Contains(address) {
		return "", fmt.Errorf("failed to calculate DNS address: CIDR too narrow: %s", n.ServiceCIDR)
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
		return nil, fmt.Errorf("can't parse service CIDR to build internal API address: %w", err)
	}

	stringifiedAddresses := make([]string, len(parsedCIDRs))
	for i, ip := range parsedCIDRs {
		apiIP, err := utilnet.GetIndexedIP(ip, 1)
		if err != nil {
			return nil, fmt.Errorf("can't build internal API address: %w", err)
		}
		stringifiedAddresses[i] = apiIP.String()
	}
	return stringifiedAddresses, nil
}

// UnmarshalJSON sets in some sane defaults when unmarshaling the data from json
func (n *Network) UnmarshalJSON(data []byte) error {
	n.Provider = "kuberouter"

	type network Network
	jc := (*network)(n)

	if err := json.Unmarshal(data, jc); err != nil {
		return err
	}

	switch n.Provider {
	case "calico":
		if n.Calico == nil {
			n.Calico = DefaultCalico()
		}
		n.KubeRouter = nil
	case "kuberouter":
		if n.KubeRouter == nil {
			n.KubeRouter = DefaultKubeRouter()
		}
		n.Calico = nil
	}

	if n.KubeProxy == nil {
		n.KubeProxy = DefaultKubeProxy()
	}

	return nil
}

// BuildServiceCIDR returns actual argument value for service cidr
func (n *Network) BuildServiceCIDR(primaryAddressFamily PrimaryAddressFamilyType) string {
	if !n.DualStack.Enabled {
		return n.ServiceCIDR
	}

	// Because Kubernetes relies on the order of the given CIDRs in dual-stack
	// mode, the CIDR whose version matches the version of the IP address the
	// API server is listening on must be specified first.
	switch primaryAddressFamily {
	case PrimaryFamilyIPv4:
		return n.ServiceCIDR + "," + n.DualStack.IPv6ServiceCIDR
	case PrimaryFamilyIPv6:
		return n.DualStack.IPv6ServiceCIDR + "," + n.ServiceCIDR
	default:
		panic(fmt.Sprintf("BuildServiceCIDR called invalid PrimaryAddressFamily %q family. This is theoretically impossible", primaryAddressFamily))
	}
}

// BuildPodCIDR returns actual argument value for pod cidr
func (n *Network) BuildPodCIDR() string {
	if n.DualStack.Enabled {
		return n.DualStack.IPv6PodCIDR + "," + n.PodCIDR
	}
	return n.PodCIDR
}
