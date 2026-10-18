// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"errors"
	"testing"

	"github.com/k0sproject/k0s/internal/featuregate"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bgpCPLB() *ControlPlaneLoadBalancingSpec {
	return &ControlPlaneLoadBalancingSpec{
		Enabled: true,
		Type:    CPLBTypeBGP,
		BGP: &BGPSpec{
			VirtualIPs: []string{"192.168.10.1/32"},
			AS:         65001,
			Peers:      []BGPPeer{{Address: "10.0.0.1", AS: 65000}},
		},
	}
}

func TestBGPSpec_Validate(t *testing.T) {
	fg := featuregate.FeatureGates{}
	require.NoError(t, fg.Set("ControlPlaneLoadBalancingBGP=true"))
	t.Cleanup(func() { featuregate.FlushDefaultFeatureGates(t) })

	t.Run("defaults", func(t *testing.T) {
		c := bgpCPLB()
		c.BGP.Peers[0].BFD = &BFDSpec{}
		require.Empty(t, c.Validate(field.NewPath("cplb")))
		assert.EqualValues(t, 90, c.BGP.HoldTimeSeconds)
		assert.EqualValues(t, 30, c.BGP.KeepaliveIntervalSeconds)
		assert.EqualValues(t, 179, c.BGP.Peers[0].Port)
		assert.Equal(t, &BFDSpec{MinimumReceiveMilliseconds: 300, MinimumTransmitMilliseconds: 300, Multiplier: 3}, c.BGP.Peers[0].BFD)
		assert.Empty(t, c.Validate(field.NewPath("cplb")), "validating again should succeed")
	})

	t.Run("iBGP", func(t *testing.T) {
		c := bgpCPLB()
		c.BGP.AS = c.BGP.Peers[0].AS
		assert.Empty(t, c.Validate(field.NewPath("cplb")))
	})

	for _, test := range []struct {
		name   string
		modify func(*ControlPlaneLoadBalancingSpec)
		err    string
	}{
		{"missing bgp", func(c *ControlPlaneLoadBalancingSpec) { c.BGP = nil }, "cplb.bgp: Required value"},
		{"keepalived", func(c *ControlPlaneLoadBalancingSpec) { c.Keepalived = &KeepalivedSpec{} }, "cplb.keepalived: Forbidden"},
		{"bgp with keepalived type", func(c *ControlPlaneLoadBalancingSpec) { c.Type = CPLBTypeKeepalived }, "cplb.bgp: Forbidden: requires type BGP"},
		{"missing AS", func(c *ControlPlaneLoadBalancingSpec) { c.BGP.AS = 0 }, "cplb.bgp.as: Required value"},
		{"IPv6 router ID", func(c *ControlPlaneLoadBalancingSpec) { c.BGP.RouterID = "fd00::1" }, "cplb.bgp.routerID: Invalid value"},
		{"both sources", func(c *ControlPlaneLoadBalancingSpec) {
			c.BGP.SourceIP, c.BGP.SourceInterface = "10.0.0.2", "eth0"
		}, "cplb.bgp.sourceInterface: Forbidden"},
		{"keepalive not shorter than hold time", func(c *ControlPlaneLoadBalancingSpec) {
			c.BGP.HoldTimeSeconds, c.BGP.KeepaliveIntervalSeconds = 9, 9
		}, "cplb.bgp.keepaliveIntervalSeconds: Invalid value"},
		{"hold time too short", func(c *ControlPlaneLoadBalancingSpec) {
			c.BGP.HoldTimeSeconds, c.BGP.KeepaliveIntervalSeconds = 2, 1
		}, "cplb.bgp.holdTimeSeconds: Invalid value"},
		{"no peers", func(c *ControlPlaneLoadBalancingSpec) { c.BGP.Peers = nil }, "cplb.bgp.peers: Required value"},
		{"BFD with multihop", func(c *ControlPlaneLoadBalancingSpec) {
			c.BGP.Peers[0].BFD, c.BGP.Peers[0].Multihop = &BFDSpec{}, true
		}, "cplb.bgp.peers[0].bfd: Forbidden"},
		{"invalid peer address", func(c *ControlPlaneLoadBalancingSpec) { c.BGP.Peers[0].Address = "router" }, "cplb.bgp.peers[0].address: Invalid value"},
		{"link-local peer address", func(c *ControlPlaneLoadBalancingSpec) { c.BGP.Peers[0].Address = "fe80::1" }, "must not be a link-local address"},
		{"missing peer AS", func(c *ControlPlaneLoadBalancingSpec) { c.BGP.Peers[0].AS = 0 }, "cplb.bgp.peers[0].as: Required value"},
		{"no virtual IPs", func(c *ControlPlaneLoadBalancingSpec) { c.BGP.VirtualIPs = nil }, "cplb.bgp.virtualIPs: Required value"},
		{"not a host prefix", func(c *ControlPlaneLoadBalancingSpec) {
			c.BGP.VirtualIPs = []string{"192.168.10.1/24"}
		}, "must be a host prefix"},
		{"no peer of the family", func(c *ControlPlaneLoadBalancingSpec) {
			c.BGP.VirtualIPs = []string{"fd02::1/128"}
		}, "no BGP peer of the same IP family"},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := bgpCPLB()
			test.modify(c)
			errs := c.Validate(field.NewPath("cplb"))
			if assert.Len(t, errs, 1, "the configuration should be rejected once") {
				assert.ErrorContains(t, errs[0], test.err)
			}
		})
	}
}

func TestBGPSpec_FeatureGate(t *testing.T) {
	fg := featuregate.FeatureGates{}
	require.NoError(t, fg.Set(""))
	t.Cleanup(func() { featuregate.FlushDefaultFeatureGates(t) })

	errs := bgpCPLB().Validate(field.NewPath("cplb"))
	if assert.Len(t, errs, 1) {
		assert.ErrorContains(t, errs[0], "feature gate ControlPlaneLoadBalancingBGP must be explicitly enabled")
	}
}

func TestBGPSpec_OnlyBindToAddress(t *testing.T) {
	fg := featuregate.FeatureGates{}
	require.NoError(t, fg.Set("ControlPlaneLoadBalancingBGP=true"))
	t.Cleanup(func() { featuregate.FlushDefaultFeatureGates(t) })

	c := DefaultClusterConfig()
	c.Spec.Network.ControlPlaneLoadBalancing = bgpCPLB()
	c.Spec.API.OnlyBindToAddress = true
	assert.ErrorContains(t, errors.Join(c.Spec.Validate()...), "BGP cannot be used in conjunction with spec.api.onlyBindToAddress")
}
