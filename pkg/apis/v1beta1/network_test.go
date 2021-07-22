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
	"testing"

	"github.com/stretchr/testify/suite"
)

type NetworkSuite struct {
	suite.Suite
}

func (s *NetworkSuite) TestAddresses() {
	s.T().Run("DNS_default_service_cidr", func(t *testing.T) {
		n := DefaultNetwork()
		dns, err := n.DNSAddress()
		s.NoError(err)
		s.Equal("10.96.0.10", dns)
	})
	s.T().Run("DNS_uses_non_default_service_cidr", func(t *testing.T) {
		n := DefaultNetwork()
		n.ServiceCIDR = "10.96.0.248/29"
		dns, err := n.DNSAddress()
		s.NoError(err)
		s.Equal("10.96.0.250", dns)
	})
	s.T().Run("Internal_api_address_default", func(t *testing.T) {
		n := DefaultNetwork()
		api, err := n.InternalAPIAddresses()
		s.NoError(err)
		s.Equal([]string{"10.96.0.1"}, api)
	})
	s.T().Run("Internal_api_address_non_default_single_stack", func(t *testing.T) {
		n := DefaultNetwork()
		n.ServiceCIDR = "10.96.0.248/29"
		api, err := n.InternalAPIAddresses()
		s.NoError(err)
		s.Equal([]string{"10.96.0.249"}, api)
	})
	s.T().Run("Internal_api_address_non_default_dual_stack", func(t *testing.T) {
		n := DefaultNetwork()
		n.ServiceCIDR = "10.96.0.248/29"
		n.DualStack.Enabled = true
		n.DualStack.IPv6ServiceCIDR = "fd00::/108"
		api, err := n.InternalAPIAddresses()
		s.NoError(err)
		s.Equal([]string{"10.96.0.249", "fd00::1"}, api)
	})

	s.T().Run("BuildServiceCIDR ordering", func(t *testing.T) {
		t.Run("single_stack_default", func(t *testing.T) {
			n := DefaultNetwork()
			s.Equal(n.ServiceCIDR, n.BuildServiceCIDR("10.96.0.249"))
		})
		t.Run("dual_stack_api_listens_on_ipv4", func(t *testing.T) {
			n := DefaultNetwork()
			n.DualStack.Enabled = true
			n.DualStack.IPv6ServiceCIDR = "fd00::/108"
			s.Equal(n.ServiceCIDR+","+n.DualStack.IPv6ServiceCIDR, n.BuildServiceCIDR("10.96.0.249"))
		})
		t.Run("dual_stack_api_listens_on_ipv6", func(t *testing.T) {
			n := DefaultNetwork()
			n.DualStack.Enabled = true
			n.DualStack.IPv6ServiceCIDR = "fd00::/108"
			s.Equal(n.DualStack.IPv6ServiceCIDR+","+n.ServiceCIDR, n.BuildServiceCIDR("fe80::cf8:3cff:fef2:c5ca"))
		})
	})
}

func (s *NetworkSuite) TestNetworkDefaults() {
	n := DefaultNetwork()

	s.Equal("kuberouter", n.Provider)
	s.NotNil(n.KubeRouter)
	s.Equal(ModeIptables, n.KubeProxy.Mode)
}

func (s *NetworkSuite) TestCalicoDefaultsAfterMashaling() {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: foobar
spec:
  network:
    provider: calico
    calico:
`

	c, err := configFromString(yamlData, k0sVars)
	s.NoError(err)
	n := c.Spec.Network

	s.Equal("calico", n.Provider)
	s.NotNil(n.Calico)
	s.Equal(4789, n.Calico.VxlanPort)
	s.Equal(0, n.Calico.MTU)
	s.Equal("vxlan", n.Calico.Mode)
}

func (s *NetworkSuite) TestKubeRouterDefaultsAfterMashaling() {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: foobar
spec:
  network:
    provider: kuberouter
    kuberouter:
`

	c, err := configFromString(yamlData, k0sVars)
	s.NoError(err)
	n := c.Spec.Network

	s.Equal("kuberouter", n.Provider)
	s.NotNil(n.KubeRouter)
	s.Nil(n.Calico)

	s.True(n.KubeRouter.AutoMTU)
	s.Equal(0, n.KubeRouter.MTU)
	s.Empty(n.KubeRouter.PeerRouterASNs)
	s.Empty(n.KubeRouter.PeerRouterIPs)
}

func (s *NetworkSuite) TestKubeProxyDefaultsAfterMashaling() {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: foobar
spec:
`

	c, err := configFromString(yamlData, k0sVars)
	s.NoError(err)
	p := c.Spec.Network.KubeProxy

	s.Equal(ModeIptables, p.Mode)
	s.False(p.Disabled)
}

func (s *NetworkSuite) TestKubeProxyDisabling() {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: foobar
spec:
  network:
    kubeProxy:
      disabled: true
`

	c, err := configFromString(yamlData, k0sVars)
	s.NoError(err)
	p := c.Spec.Network.KubeProxy

	s.True(p.Disabled)
}

func (s *NetworkSuite) TestValidation() {
	s.T().Run("defaults_are_valid", func(t *testing.T) {
		n := DefaultNetwork()

		s.Nil(n.Validate())
	})

	s.T().Run("invalid_provider", func(t *testing.T) {
		n := DefaultNetwork()
		n.Provider = "foobar"

		errors := n.Validate()
		s.NotNil(errors)
		s.Len(errors, 1)
	})

	s.T().Run("invalid_pod_cidr", func(t *testing.T) {
		n := DefaultNetwork()
		n.PodCIDR = "foobar"

		errors := n.Validate()
		s.NotNil(errors)
		s.Len(errors, 1)
		s.Contains(errors[0].Error(), "invalid pod CIDR")
	})

	s.T().Run("invalid_service_cidr", func(t *testing.T) {
		n := DefaultNetwork()
		n.ServiceCIDR = "foobar"

		errors := n.Validate()
		s.NotNil(errors)
		s.Len(errors, 1)
		s.Contains(errors[0].Error(), "invalid service CIDR")
	})

	s.T().Run("invalid_ipv6_service_cidr", func(t *testing.T) {
		n := DefaultNetwork()
		n.Calico = DefaultCalico()
		n.Calico.Mode = "bird"
		n.DualStack = DefaultDualStack()
		n.DualStack.Enabled = true
		n.KubeProxy.Mode = "ipvs"
		n.DualStack.IPv6PodCIDR = "fd00::/108"
		n.DualStack.IPv6ServiceCIDR = "foobar"

		errors := n.Validate()
		s.NotNil(errors)
		s.Len(errors, 1)
		s.Contains(errors[0].Error(), "invalid service IPv6 CIDR")
	})

	s.T().Run("invalid_ipv6_pod_cidr", func(t *testing.T) {
		n := DefaultNetwork()
		n.Calico = DefaultCalico()
		n.Calico.Mode = "bird"
		n.DualStack = DefaultDualStack()
		n.DualStack.IPv6PodCIDR = "foobar"
		n.DualStack.IPv6ServiceCIDR = "fd00::/108"
		n.DualStack.Enabled = true
		n.KubeProxy.Mode = "ipvs"

		errors := n.Validate()
		s.NotNil(errors)
		s.Len(errors, 1)
		s.Contains(errors[0].Error(), "invalid pod IPv6 CIDR")
	})

	s.T().Run("invalid_mode_for_kube_proxy", func(t *testing.T) {
		n := DefaultNetwork()
		n.KubeProxy.Mode = "foobar"

		errors := n.Validate()
		s.NotNil(errors)
		s.Len(errors, 1)
		s.Contains(errors[0].Error(), "unsupported mode")
	})

	s.T().Run("invalid_proxy_mode_for_dualstack", func(t *testing.T) {
		n := DefaultNetwork()
		n.Calico = DefaultCalico()
		n.Calico.Mode = "bird"
		n.DualStack = DefaultDualStack()
		n.DualStack.Enabled = true
		n.KubeProxy.Mode = "iptables"
		n.DualStack.IPv6PodCIDR = "fd00::/108"
		n.DualStack.IPv6ServiceCIDR = "fd01::/108"

		errors := n.Validate()
		s.NotNil(errors)
		s.Len(errors, 1)
		s.Contains(errors[0].Error(), "dual-stack requires kube-proxy in ipvs mode")
	})
}

func TestNetworkSuite(t *testing.T) {
	ns := &NetworkSuite{}

	suite.Run(t, ns)
}
