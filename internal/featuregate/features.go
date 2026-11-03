// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package featuregate

const (
	// IPv6SingleStack enables single stack IPv6 support in k0s
	IPv6SingleStack Feature = "IPv6SingleStack"

	// ControlPlaneLoadBalancingBGP enables the BGP type of control plane load balancing
	ControlPlaneLoadBalancingBGP Feature = "ControlPlaneLoadBalancingBGP"
)

func defaultFeatureMap() map[Feature]FeatureGate {
	return map[Feature]FeatureGate{
		IPv6SingleStack:              {Stage: Alpha},
		ControlPlaneLoadBalancingBGP: {Stage: Alpha},
	}
}
