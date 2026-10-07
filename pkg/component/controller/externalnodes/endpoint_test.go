// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package externalnodes

import (
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestControlPlaneEndpoint(t *testing.T) {
	cplb := func(vips ...string) *v1beta1.ControlPlaneLoadBalancingSpec {
		return &v1beta1.ControlPlaneLoadBalancingSpec{
			Enabled: true, Type: v1beta1.CPLBTypeKeepalived,
			Keepalived: &v1beta1.KeepalivedSpec{VRRPInstances: v1beta1.VRRPInstances{{VirtualIPs: vips}}},
		}
	}
	for _, test := range []struct {
		cplb            *v1beta1.ControlPlaneLoadBalancingSpec
		name            string
		externalAddress string
		want            string
		leaderAddress   bool
	}{
		{nil, "controller address", "", "10.0.0.1:6443", true},
		{nil, "external address", "k8s.example.com", "k8s.example.com:6443", false},
		{cplb("10.0.0.100/24"), "virtual IP", "", "10.0.0.100:6443", false},
		{cplb("fd00::100/64"), "IPv6 virtual IP", "", "[fd00::100]:6443", false},
		{cplb("10.0.0.100/24", "10.0.0.101/24"), "first virtual IP", "", "10.0.0.100:6443", false},
		{cplb("10.0.0.100/24"), "external address over virtual IP", "k8s.example.com", "k8s.example.com:6443", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			spec := v1beta1.DefaultClusterSpec()
			spec.API.Address = "10.0.0.1"
			spec.API.ExternalAddress = test.externalAddress
			spec.Network.ControlPlaneLoadBalancing = test.cplb
			endpoint, err := ControlPlaneEndpoint(spec)
			require.NoError(t, err)
			assert.Equal(t, test.want, endpoint)
			assert.Equal(t, test.leaderAddress, UsesLeaderAddress(spec))
		})
	}

	spec := v1beta1.DefaultClusterSpec()
	spec.API.Address = "["
	_, err := ControlPlaneEndpoint(spec)
	assert.ErrorContains(t, err, "failed to determine the control plane endpoint")
}

func TestControlPlaneVirtualIP(t *testing.T) {
	keepalived := func(vips ...string) *v1beta1.KeepalivedSpec {
		return &v1beta1.KeepalivedSpec{VRRPInstances: v1beta1.VRRPInstances{{VirtualIPs: vips}}}
	}
	for _, test := range []struct {
		network *v1beta1.Network
		name    string
		want    string
	}{
		{nil, "no network", ""},
		{&v1beta1.Network{}, "no load balancing", ""},
		{&v1beta1.Network{ControlPlaneLoadBalancing: &v1beta1.ControlPlaneLoadBalancingSpec{Keepalived: keepalived("10.0.0.100/24")}}, "disabled", ""},
		{&v1beta1.Network{ControlPlaneLoadBalancing: &v1beta1.ControlPlaneLoadBalancingSpec{Enabled: true}}, "no keepalived", ""},
		{&v1beta1.Network{ControlPlaneLoadBalancing: &v1beta1.ControlPlaneLoadBalancingSpec{Enabled: true, Keepalived: keepalived("invalid", "10.0.0.100/24")}}, "invalid entries skipped", "10.0.0.100"},
		{&v1beta1.Network{ControlPlaneLoadBalancing: &v1beta1.ControlPlaneLoadBalancingSpec{Enabled: true, Keepalived: keepalived("fd00::100/64")}}, "IPv6", "fd00::100"},
		{&v1beta1.Network{ControlPlaneLoadBalancing: &v1beta1.ControlPlaneLoadBalancingSpec{Enabled: true, Keepalived: keepalived("invalid")}}, "only invalid entries", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			vip := controlPlaneVirtualIP(test.network)
			if test.want == "" {
				assert.False(t, vip.IsValid())
			} else {
				assert.Equal(t, test.want, vip.String())
			}
		})
	}
}
