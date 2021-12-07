package cluster

import (
	"net"

	"github.com/docker/docker/api/types/network"
	"github.com/weaveworks/footloose/pkg/ignite"
)

const (
	ipv4Length = 32
)

// NewRuntimeNetworks returns a slice of networks
func NewRuntimeNetworks(networks map[string]*network.EndpointSettings) []*RuntimeNetwork {
	rnList := make([]*RuntimeNetwork, 0, len(networks))
	for key, value := range networks {
		mask := net.CIDRMask(value.IPPrefixLen, ipv4Length)
		maskIP := net.IP(mask).String()
		rnNetwork := &RuntimeNetwork{
			Name:    key,
			IP:      value.IPAddress,
			Mask:    maskIP,
			Gateway: value.Gateway,
		}
		rnList = append(rnList, rnNetwork)
	}
	return rnList
}

// NewIgniteRuntimeNetwork creates reports network status for the ignite backend.
func NewIgniteRuntimeNetwork(status *ignite.Status) []*RuntimeNetwork {
	networks := make([]*RuntimeNetwork, 0, len(status.IpAddresses))
	for _, ip := range status.IpAddresses {
		networks = append(networks, &RuntimeNetwork{
			IP: ip,
		})
	}

	return networks
}

// RuntimeNetwork contains information about the network
type RuntimeNetwork struct {
	// Name of the network
	Name string `json:"name,omitempty"`
	// IP of the container
	IP string `json:"ip,omitempty"`
	// Mask of the network
	Mask string `json:"mask,omitempty"`
	// Gateway of the network
	Gateway string `json:"gateway,omitempty"`
}
