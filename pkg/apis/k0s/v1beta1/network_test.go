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
	"testing"

	"k8s.io/utils/ptr"

	"github.com/stretchr/testify/suite"
)

type NetworkSuite struct {
	suite.Suite
}

func (s *NetworkSuite) TestAddresses() {
	s.Run("DNS_default_service_cidr", func() {
		n := DefaultNetwork()
		dns, err := n.DNSAddress()
		s.Require().NoError(err)
		s.Equal("10.96.0.10", dns)
	})
	s.Run("DNS_uses_non_default_service_cidr", func() {
		n := DefaultNetwork()
		n.ServiceCIDR = "10.96.0.248/29"
		dns, err := n.DNSAddress()
		s.Require().NoError(err)
		s.Equal("10.96.0.250", dns)
	})
	s.Run("Internal_api_address_default", func() {
		n := DefaultNetwork()
		api, err := n.InternalAPIAddresses()
		s.Require().NoError(err)
		s.Equal([]string{"10.96.0.1"}, api)
	})
	s.Run("Internal_api_address_non_default_single_stack", func() {
		n := DefaultNetwork()
		n.ServiceCIDR = "10.96.0.248/29"
		api, err := n.InternalAPIAddresses()
		s.Require().NoError(err)
		s.Equal([]string{"10.96.0.249"}, api)
	})
	s.Run("Internal_api_address_non_default_dual_stack", func() {
		n := DefaultNetwork()
		n.ServiceCIDR = "10.96.0.248/29"
		n.DualStack.Enabled = true
		n.DualStack.IPv6ServiceCIDR = "fd00::/108"
		api, err := n.InternalAPIAddresses()
		s.Require().NoError(err)
		s.Equal([]string{"10.96.0.249", "fd00::1"}, api)
	})

	s.Run("BuildServiceCIDR ordering", func() {
		s.Run("single_stack_default", func() {
			n := DefaultNetwork()
			s.Equal(n.ServiceCIDR, n.BuildServiceCIDR("10.96.0.249"))
		})
		s.Run("dual_stack_api_listens_on_ipv4", func() {
			n := DefaultNetwork()
			n.DualStack.Enabled = true
			n.DualStack.IPv6ServiceCIDR = "fd00::/108"
			s.Equal(n.ServiceCIDR+","+n.DualStack.IPv6ServiceCIDR, n.BuildServiceCIDR("10.96.0.249"))
		})
		s.Run("dual_stack_api_listens_on_ipv6", func() {
			n := DefaultNetwork()
			n.DualStack.Enabled = true
			n.DualStack.IPv6ServiceCIDR = "fd00::/108"
			s.Equal(n.DualStack.IPv6ServiceCIDR+","+n.ServiceCIDR, n.BuildServiceCIDR("fe80::cf8:3cff:fef2:c5ca"))
		})
	})
}

func (s *NetworkSuite) TestDomainMarshaling() {
	yamlData := `
spec:
  storage:
    type: kine
  network:
    clusterDomain: something.local
`
	c, err := ConfigFromString(yamlData)
	s.Require().NoError(err)
	n := c.Spec.Network
	s.Equal("kuberouter", n.Provider)
	s.NotNil(n.KubeRouter)
	s.Equal("something.local", n.ClusterDomain)
}

func (s *NetworkSuite) TestNetworkDefaults() {
	n := DefaultNetwork()

	s.Equal("kuberouter", n.Provider)
	s.NotNil(n.KubeRouter)
	s.Equal(ModeIptables, n.KubeProxy.Mode)
	s.Equal("cluster.local", n.ClusterDomain)
}

func (s *NetworkSuite) TestCalicoDefaultsAfterMashaling() {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  network:
    provider: calico
    calico:
`

	c, err := ConfigFromString(yamlData)
	s.Require().NoError(err)
	n := c.Spec.Network

	s.Equal("calico", n.Provider)
	s.NotNil(n.Calico)
	s.Equal(4789, n.Calico.VxlanPort)
	s.Equal(1450, n.Calico.MTU)
	s.Equal(CalicoModeVXLAN, n.Calico.Mode)
}

func (s *NetworkSuite) TestKubeRouterDefaultsAfterMashaling() {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  network:
    provider: kuberouter
    kuberouter:
`

	c, err := ConfigFromString(yamlData)
	s.Require().NoError(err)
	n := c.Spec.Network

	s.Equal("kuberouter", n.Provider)
	s.NotNil(n.KubeRouter)
	s.Nil(n.Calico)

	s.Equal(ptr.To(true), n.KubeRouter.AutoMTU)
	s.Equal(0, n.KubeRouter.MTU)
	s.Empty(n.KubeRouter.PeerRouterASNs)
	s.Empty(n.KubeRouter.PeerRouterIPs)
}

func (s *NetworkSuite) TestKubeProxyDefaultsAfterMashaling() {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
`

	c, err := ConfigFromString(yamlData)
	s.Require().NoError(err)
	p := c.Spec.Network.KubeProxy

	s.Equal(ModeIptables, p.Mode)
	s.False(p.Disabled)
}

func (s *NetworkSuite) TestKubeProxyDisabling() {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  network:
    kubeProxy:
      disabled: true
`

	c, err := ConfigFromString(yamlData)
	s.Require().NoError(err)
	p := c.Spec.Network.KubeProxy

	s.True(p.Disabled)
}

func (s *NetworkSuite) TestValidation() {
	s.Run("defaults_are_valid", func() {
		n := DefaultNetwork()

		s.Nil(n.Validate())
	})

	s.Run("invalid_provider", func() {
		n := DefaultNetwork()
		n.Provider = "foobar"

		errors := n.Validate()
		s.NotNil(errors)
		s.Len(errors, 1)
	})

	s.Run("invalid_pod_cidr", func() {
		n := DefaultNetwork()
		n.PodCIDR = "foobar"

		errors := n.Validate()
		if s.Len(errors, 1) {
			s.ErrorContains(errors[0], `Invalid value: "foobar": invalid CIDR address`)
		}
	})

	s.Run("invalid_service_cidr", func() {
		n := DefaultNetwork()
		n.ServiceCIDR = "foobar"

		errors := n.Validate()
		if s.Len(errors, 1) {
			s.ErrorContains(errors[0], `Invalid value: "foobar": invalid CIDR address`)
		}
	})

	s.Run("invalid_cluster_domain", func() {
		n := DefaultNetwork()
		n.ClusterDomain = ".invalid-cluster-domain"

		errors := n.Validate()
		if s.Len(errors, 1) {
			s.ErrorContains(errors[0], `clusterDomain: Invalid value: ".invalid-cluster-domain": invalid DNS name`)
		}
	})

	s.Run("invalid_ipv6_service_cidr", func() {
		n := DefaultNetwork()
		n.Calico = DefaultCalico()
		n.Calico.Mode = CalicoModeBIRD
		n.DualStack = DefaultDualStack()
		n.DualStack.Enabled = true
		n.KubeProxy.Mode = "ipvs"
		n.DualStack.IPv6PodCIDR = "fd00::/108"
		n.DualStack.IPv6ServiceCIDR = "foobar"

		errors := n.Validate()
		if s.Len(errors, 1) {
			s.ErrorContains(errors[0], `dualStack.IPv6serviceCIDR: Invalid value: "foobar": invalid CIDR address`)
		}
	})

	s.Run("invalid_ipv6_pod_cidr", func() {
		n := DefaultNetwork()
		n.Calico = DefaultCalico()
		n.Calico.Mode = CalicoModeBIRD
		n.DualStack = DefaultDualStack()
		n.DualStack.IPv6PodCIDR = "foobar"
		n.DualStack.IPv6ServiceCIDR = "fd00::/108"
		n.DualStack.Enabled = true
		n.KubeProxy.Mode = "ipvs"

		errors := n.Validate()
		if s.Len(errors, 1) {
			s.ErrorContains(errors[0], `Invalid value: "foobar": invalid CIDR address`)
		}
	})

	s.Run("invalid_mode_for_kube_proxy", func() {
		n := DefaultNetwork()
		n.KubeProxy.Mode = "foobar"

		errors := n.Validate()
		if s.Len(errors, 1) {
			s.ErrorContains(errors[0], "unsupported mode foobar for kubeProxy config")
		}
	})

	s.Run("valid_proxy_disabled_for_dualstack", func() {
		n := DefaultNetwork()
		n.Calico = DefaultCalico()
		n.Calico.Mode = CalicoModeBIRD
		n.DualStack = DefaultDualStack()
		n.DualStack.Enabled = true
		n.KubeProxy.Disabled = true
		n.KubeProxy.Mode = "iptables"
		n.DualStack.IPv6PodCIDR = "fd00::/108"
		n.DualStack.IPv6ServiceCIDR = "fd01::/108"

		errors := n.Validate()
		s.Nil(errors)
	})
}

func TestNetworkSuite(t *testing.T) {
	ns := &NetworkSuite{}

	suite.Run(t, ns)
}
