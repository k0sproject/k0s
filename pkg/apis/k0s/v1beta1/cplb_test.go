/*
Copyright 2024 k0s authors

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
	"k8s.io/utils/ptr"
)

type CPLBSuite struct {
	suite.Suite
}

func (s *CPLBSuite) TestValidateVRRPInstances() {

	tests := []struct {
		name          string
		vrrps         []VRRPInstance
		expectedVRRPs []VRRPInstance
		wantErr       bool
	}{
		{
			name: "Set expected defaults",
			vrrps: []VRRPInstance{
				{
					VirtualIPs: []string{"192.168.1.1/24"},
					AuthPass:   "123456",
				},
				{
					VirtualIPs: []string{"192.168.7.1/24"},
					AuthPass:   "12345678",
				},
			},
			expectedVRRPs: []VRRPInstance{
				{
					Name:            "k0s-vip-0",
					VirtualRouterID: ptr.To(int32(defaultVirtualRouterID)),
					Interface:       "fake-nic-0",
					VirtualIPs:      []string{"192.168.1.1/24"},
					AdvertInterval:  ptr.To(int32(defaultAdvertInterval)),
					AuthPass:        "123456",
				},
				{
					Name:            "k0s-vip-1",
					VirtualRouterID: ptr.To(int32(defaultVirtualRouterID + 1)),
					Interface:       "fake-nic-0",
					VirtualIPs:      []string{"192.168.1.1/24"},
					AdvertInterval:  ptr.To(int32(defaultAdvertInterval)),
					AuthPass:        "12345678",
				},
			},
			wantErr: false,
		},
		{
			name: "valid instance no overrides",
			vrrps: []VRRPInstance{
				{
					Name:            "test",
					VirtualRouterID: ptr.To(int32(1)),
					Interface:       "eth0",
					VirtualIPs:      []string{"192.168.1.1/24"},
					AdvertInterval:  ptr.To(int32(1)),
					AuthPass:        "123456",
				},
			},
			expectedVRRPs: []VRRPInstance{
				{
					Name:            "test",
					VirtualRouterID: ptr.To(int32(1)),
					Interface:       "eth0",
					VirtualIPs:      []string{"192.168.1.1/24"},
					AdvertInterval:  ptr.To(int32(1)),
					AuthPass:        "123456",
				},
			},
			wantErr: false,
		}, {
			name: "No password",
			vrrps: []VRRPInstance{
				{
					Name:            "test",
					VirtualRouterID: ptr.To(int32(1)),
					Interface:       "eth0",
					VirtualIPs:      []string{"192.168.1.1/24"},
					AdvertInterval:  ptr.To(int32(1)),
				},
			},
			wantErr: true,
		}, {
			name: "Password too long",
			vrrps: []VRRPInstance{
				{
					VirtualIPs: []string{"192.168.1.1/24"},
					AuthPass:   "012345678",
				},
			},
			wantErr: true,
		}, {
			name: "Invalid CIDR",
			vrrps: []VRRPInstance{
				{
					VirtualIPs: []string{"192.168.1.1"},
					AuthPass:   "123456",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			elb := &ControlPlaneLoadBalancingSpec{
				VRRPInstances: tt.vrrps,
			}
			err := elb.ValidateVRRPInstances(returnNIC)
			if tt.wantErr {
				s.Require().Errorf(err, "Test case %s expected error. Got none", tt.name)
			} else {
				s.Require().NoErrorf(err, "Test case %s expected no error. Got: %v", tt.name, err)
				s.T().Log(elb.VRRPInstances)
				s.Require().Equal(len(tt.expectedVRRPs), len(elb.VRRPInstances), "Expected and actual VRRPInstances length mismatch")
				for i := 0; i < len(tt.expectedVRRPs); i++ {
					s.Require().Equal(tt.expectedVRRPs[i].Name, elb.VRRPInstances[i].Name, "Name mismatch")
					s.Require().Equal(tt.expectedVRRPs[i].Interface, elb.VRRPInstances[i].Interface, "Interface mismatch")
					s.Require().Equal(*tt.expectedVRRPs[i].VirtualRouterID, *elb.VRRPInstances[i].VirtualRouterID, "Virtual router ID mismatch")
					s.Require().Equal(*tt.expectedVRRPs[i].AdvertInterval, *elb.VRRPInstances[i].AdvertInterval, "Virtual router ID mismatch")
				}
			}
		})
	}
}

func returnNIC() (string, error) {
	return "fake-nic-0", nil
}

func TestCPLBSuite(t *testing.T) {
	cplbSuite := &CPLBSuite{}

	suite.Run(t, cplbSuite)
}
