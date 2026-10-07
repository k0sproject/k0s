// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package externalnodes

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

// ControlPlaneEndpoint returns the host and port of the API server for nodes without k0s.
// Without an external address, a virtual IP of control plane load balancing stays reachable on failover.
func ControlPlaneEndpoint(spec *v1beta1.ClusterSpec) (string, error) {
	api := spec.API
	if api.ExternalAddress == "" {
		if vip := controlPlaneVirtualIP(spec.Network); vip.IsValid() {
			return net.JoinHostPort(vip.String(), strconv.Itoa(api.Port)), nil
		}
	}
	hostPort, err := api.APIServerHostPort()
	if err != nil {
		return "", fmt.Errorf("failed to determine the control plane endpoint: %w", err)
	}
	return hostPort.String(), nil
}

// UsesLeaderAddress reports whether the control plane endpoint is the address of the leading controller.
// That address changes with the leader in HA setups.
func UsesLeaderAddress(spec *v1beta1.ClusterSpec) bool {
	return spec.API.ExternalAddress == "" && !controlPlaneVirtualIP(spec.Network).IsValid()
}

// controlPlaneVirtualIP returns the first virtual IP of control plane load balancing, if enabled.
// It's in the API server certificate, and the API server port on it is load balanced.
func controlPlaneVirtualIP(network *v1beta1.Network) netip.Addr {
	if network == nil {
		return netip.Addr{}
	}
	cplb := network.ControlPlaneLoadBalancing
	if cplb == nil || !cplb.Enabled || cplb.Keepalived == nil {
		return netip.Addr{}
	}
	for _, instance := range cplb.Keepalived.VRRPInstances {
		for _, vip := range instance.VirtualIPs {
			if prefix, err := netip.ParsePrefix(vip); err == nil {
				return prefix.Addr()
			}
		}
	}
	return netip.Addr{}
}
