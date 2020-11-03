/*
Copyright 2020 Mirantis, Inc.

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

func (s *NetworkSuite) TestDNSAddress() {
	n := DefaultNetwork()
	dns, err := n.DNSAddress()
	s.NoError(err)
	s.Equal("10.96.0.10", dns)

	api, err := n.InternalAPIAddress()
	s.NoError(err)
	s.Equal("10.96.0.1", api)

	n.ServiceCIDR = "10.96.0.248/29"
	dns, err = n.DNSAddress()
	s.NoError(err)
	s.Equal("10.96.0.250", dns)

	api, err = n.InternalAPIAddress()
	s.NoError(err)
	s.Equal("10.96.0.249", api)

}

func (s *NetworkSuite) TestCalicoDefaults() {
	n := DefaultNetwork()

	s.Equal("calico", n.Provider)
	s.NotNil(n.Calico)
	s.Equal(4789, n.Calico.VxlanPort)
	s.Equal(1450, n.Calico.MTU)
	s.Equal("vxlan", n.Calico.Mode)
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

	c, err := fromYaml(s.T(), yamlData)
	s.NoError(err)
	n := c.Spec.Network

	s.Equal("calico", n.Provider)
	s.NotNil(n.Calico)
	s.Equal(4789, n.Calico.VxlanPort)
	s.Equal(1450, n.Calico.MTU)
	s.Equal("vxlan", n.Calico.Mode)
}

func TestNetworkSuite(t *testing.T) {
	ns := &NetworkSuite{}

	suite.Run(t, ns)
}
