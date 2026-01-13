// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/k0sproject/k0s/pkg/featuregate"
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
	s.Run("DNS_service_cidr_too_narrow", func() {
		n := Network{ServiceCIDR: "192.168.178.0/31"}
		dns, err := n.DNSAddress()
		s.Empty(dns)
		s.ErrorContains(err, "failed to calculate DNS address: CIDR too narrow: 192.168.178.0/31")
	})
	s.Run("DNS_uses_v6_service_cidr", func() {
		n := Network{ServiceCIDR: "fd00:abcd:1234::/64"}
		dns, err := n.DNSAddress()
		s.NoError(err)
		s.Equal("fd00:abcd:1234::a", dns)
	})
	s.Run("DNS_uses_v6_small_service_cidr", func() {
		n := Network{ServiceCIDR: "fd00::/126"}
		dns, err := n.DNSAddress()
		s.NoError(err)
		s.Equal("fd00::2", dns)
	})
	s.Run("DNS_service_v6_cidr_too_narrow", func() {
		n := Network{ServiceCIDR: "fd00::/127"}
		dns, err := n.DNSAddress()
		s.Empty(dns)
		s.ErrorContains(err, "failed to calculate DNS address: CIDR too narrow: fd00::/127")
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
			s.Equal(n.ServiceCIDR+","+n.DualStack.IPv6ServiceCIDR, n.BuildServiceCIDR(PrimaryFamilyIPv4))
		})
		s.Run("dual_stack_api_listens_on_ipv6", func() {
			n := DefaultNetwork()
			n.DualStack.Enabled = true
			n.DualStack.IPv6ServiceCIDR = "fd00::/108"
			s.Equal(n.DualStack.IPv6ServiceCIDR+","+n.ServiceCIDR, n.BuildServiceCIDR(PrimaryFamilyIPv6))
		})
	})
}

func (s *NetworkSuite) TestDomainMarshaling() {
	yamlData := []byte(`
spec:
  storage:
    type: kine
  network:
    clusterDomain: something.local
`)
	c, err := ConfigFromBytes(yamlData)
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

func (s *NetworkSuite) TestCalicoDefaultsAfterMarshaling() {
	yamlData := []byte(`
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  network:
    provider: calico
    calico:
`)

	c, err := ConfigFromBytes(yamlData)
	s.Require().NoError(err)
	n := c.Spec.Network

	s.Equal("calico", n.Provider)
	s.NotNil(n.Calico)
	s.Nil(n.KubeRouter)
	s.Equal(4789, n.Calico.VxlanPort)
	s.Equal(1450, n.Calico.MTU)
	s.Equal(CalicoModeVXLAN, n.Calico.Mode)
}

func (s *NetworkSuite) TestCalicoConfigMarshaling() {
	yamlData := []byte(`
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  network:
    provider: calico
    calico:
      mode: vxlan
      mtu: 1550
      overlay: Never
      vxlanPort: 4700
`)

	c, err := ConfigFromBytes(yamlData)
	s.Require().NoError(err)
	n := c.Spec.Network

	s.Equal("calico", n.Provider)
	s.NotNil(n.Calico)
	s.Nil(n.KubeRouter)
	s.Equal(4700, n.Calico.VxlanPort)
	s.Equal(1550, n.Calico.MTU)
	s.Equal(CalicoModeVXLAN, n.Calico.Mode)
	s.Equal("Never", n.Calico.Overlay)
}

func (s *NetworkSuite) TestKubeRouterDefaultsAfterMarshaling() {
	yamlData := []byte(`
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  network:
    provider: kuberouter
    kuberouter:
`)

	c, err := ConfigFromBytes(yamlData)
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

func (s *NetworkSuite) TestKubeRouterConfigMarshaling() {
	yamlData := []byte(`
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  network:
    provider: kuberouter
    kuberouter:
      autoMTU: false
      mtu: 1500
`)

	c, err := ConfigFromBytes(yamlData)
	s.Require().NoError(err)
	n := c.Spec.Network

	s.Equal("kuberouter", n.Provider)
	s.NotNil(n.KubeRouter)
	s.Nil(n.Calico)

	s.Equal(ptr.To(false), n.KubeRouter.AutoMTU)
	s.Equal(1500, n.KubeRouter.MTU)
}

func (s *NetworkSuite) TestKubeProxyDefaultsAfterMarshaling() {
	yamlData := []byte(`
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
`)

	c, err := ConfigFromBytes(yamlData)
	s.Require().NoError(err)
	p := c.Spec.Network.KubeProxy

	s.Equal(ModeIptables, p.Mode)
	s.False(p.Disabled)
}

func (s *NetworkSuite) TestKubeProxyDisabling() {
	yamlData := []byte(`
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  network:
    kubeProxy:
      disabled: true
`)

	c, err := ConfigFromBytes(yamlData)
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

	s.Run("invalid_ipv6_service_cidr_prefix_too_large", func() {
		fg := featuregate.FeatureGates{}
		s.NoError(fg.Set("IPv6SingleStack=true"))
		defer func() { featuregate.FlushDefaultFeatureGates(s.T()) }()

		n := DefaultNetwork()
		n.PodCIDR = "fd00::/108"
		n.ServiceCIDR = "fd01::/120"

		errors := n.Validate()
		if s.Len(errors, 1) {
			s.ErrorContains(errors[0], "IPv6 service CIDR prefix must be <= 108")
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

	s.Run("dualstack_ipv6_podcidr_rejects_ipv4", func() {
		n := DefaultNetwork()
		n.DualStack.Enabled = true
		n.PodCIDR = "10.244.0.0/16"
		n.ServiceCIDR = "10.96.0.0/12"
		n.DualStack.IPv6PodCIDR = "10.0.0.0/24"
		n.DualStack.IPv6ServiceCIDR = "fd01::/108"

		errs := n.Validate()
		s.NotEmpty(errs)
		s.ErrorContains(errs[0], "must be an IPv6 CIDR")
	})

	s.Run("dualstack_ipv6_servicecidr_rejects_ipv4", func() {
		n := DefaultNetwork()
		n.DualStack.Enabled = true
		n.PodCIDR = "10.244.0.0/16"
		n.ServiceCIDR = "10.96.0.0/12"
		n.DualStack.IPv6PodCIDR = "fd00::/108"
		n.DualStack.IPv6ServiceCIDR = "10.96.0.0/12"

		errs := n.Validate()
		s.NotEmpty(errs)
		s.ErrorContains(errs[0], "must be an IPv6 CIDR")
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

	s.Run("invalid_address_family", func() {
		n := DefaultNetwork()
		for _, af := range []PrimaryAddressFamilyType{PrimaryFamilyUnknown, PrimaryFamilyIPv4, PrimaryFamilyIPv6} {
			n.PrimaryAddressFamily = af
			errors := n.Validate()
			s.Nil(errors)
		}
		n.PrimaryAddressFamily = PrimaryAddressFamilyType("IPv5")
		errors := n.Validate()
		if s.Len(errors, 1) {
			s.ErrorContains(errors[0], "Unsupported value")
		}
	})

	s.Run("invalid_pod_cidr_service_cidr_protocol_mismatch", func() {
		n := DefaultNetwork()
		n.ServiceCIDR = "fd01::/108"
		errors := n.Validate()
		if s.Len(errors, 1) {
			s.ErrorContains(errors[0], "podCIDR and serviceCIDR must be both IPv4 or IPv6")
		}
	})

	s.Run("valid_single_stack_ipv6", func() {
		fg := featuregate.FeatureGates{}
		s.NoError(fg.Set("IPv6SingleStack=true"), "Expected no error when enabling IPv6SingleStack feature gate")
		defer func() { featuregate.FlushDefaultFeatureGates(s.T()) }()

		n := DefaultNetwork()
		n.PodCIDR = "fd00::/108"
		n.ServiceCIDR = "fd01::/108"
		errors := n.Validate()
		s.Nil(errors, "Expected no errors for valid single stack IPv6 CIDRs")
	})

	s.Run("invalid_single_stack_ipv6_missing_feature_gates", func() {
		fg := featuregate.FeatureGates{}
		s.NoError(fg.Set(""), "Expected no error when setting empty feature gates")
		defer func() { featuregate.FlushDefaultFeatureGates(s.T()) }()

		n := DefaultNetwork()
		n.PodCIDR = "fd00::/108"
		n.ServiceCIDR = "fd01::/108"
		errors := n.Validate()
		if s.Len(errors, 1) {
			s.ErrorContains(errors[0], "feature gate IPv6SingleStack must be explicitly enabled to use IPv6 single stack")
		}
	})

	s.Run("invalid_dual_stack_ipv6_dualstack_CIDRs", func() {
		n := DefaultNetwork()
		n.DualStack = DefaultDualStack()
		n.DualStack.Enabled = true
		n.DualStack.IPv6PodCIDR = "fd00::/108"
		n.DualStack.IPv6ServiceCIDR = "fd01::/108"
		n.PodCIDR = "fd00::/108"
		n.ServiceCIDR = "fd01::/108"
		errors := n.Validate()
		if s.Len(errors, 2) {
			s.ErrorContains(errors[0], "if DualStack is enabled, podCIDR must be IPv4")
			s.ErrorContains(errors[1], "if DualStack is enabled, serviceCIDR must be IPv4")
		}
	})

	s.Run("invalid_both_kubeproxy_and_kuberouter_service_proxy_enabled", func() {
		n := DefaultNetwork()
		n.Provider = "kuberouter"
		n.KubeRouter = DefaultKubeRouter()
		n.KubeRouter.ExtraArgs = map[string]string{
			"run-service-proxy": "true",
		}
		n.KubeProxy.Disabled = false

		errors := n.Validate()
		if s.Len(errors, 1) {
			s.ErrorContains(errors[0], "cannot enable kube-router service proxy when kube-proxy is enabled")
		}
	})

	s.Run("valid_kuberouter_service_proxy_with_kubeproxy_disabled", func() {
		n := DefaultNetwork()
		n.Provider = "kuberouter"
		n.KubeRouter = DefaultKubeRouter()
		n.KubeRouter.ExtraArgs = map[string]string{
			"run-service-proxy": "true",
		}
		n.KubeProxy.Disabled = true

		errors := n.Validate()
		s.Nil(errors)
	})

	s.Run("valid_kubeproxy_enabled_without_kuberouter_service_proxy", func() {
		n := DefaultNetwork()
		n.Provider = "kuberouter"
		n.KubeRouter = DefaultKubeRouter()
		// No run-service-proxy set
		n.KubeProxy.Disabled = false

		errors := n.Validate()
		s.Nil(errors)
	})
}

func TestNetworkSuite(t *testing.T) {
	ns := &NetworkSuite{}

	suite.Run(t, ns)
}
