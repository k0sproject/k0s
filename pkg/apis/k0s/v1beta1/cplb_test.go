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
	"time"

	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
					VirtualRouterID:       defaultVirtualRouterID,
					Interface:             "fake-nic-0",
					VirtualIPs:            []string{"192.168.1.1/24"},
					AdvertIntervalSeconds: defaultAdvertIntervalSeconds,
					AuthPass:              "123456",
				},
				{
					VirtualRouterID:       defaultVirtualRouterID + 1,
					Interface:             "fake-nic-0",
					VirtualIPs:            []string{"192.168.1.1/24"},
					AdvertIntervalSeconds: defaultAdvertIntervalSeconds,
					AuthPass:              "12345678",
				},
			},
			wantErr: false,
		},
		{
			name: "valid instance no overrides",
			vrrps: []VRRPInstance{
				{
					VirtualRouterID:       1,
					Interface:             "eth0",
					VirtualIPs:            []string{"192.168.1.1/24"},
					AdvertIntervalSeconds: 1,
					AuthPass:              "123456",
				},
			},
			expectedVRRPs: []VRRPInstance{
				{
					VirtualRouterID:       1,
					Interface:             "eth0",
					VirtualIPs:            []string{"192.168.1.1/24"},
					AdvertIntervalSeconds: 1,
					AuthPass:              "123456",
				},
			},
			wantErr: false,
		}, {
			name: "No password",
			vrrps: []VRRPInstance{
				{
					VirtualRouterID:       1,
					Interface:             "eth0",
					VirtualIPs:            []string{"192.168.1.1/24"},
					AdvertIntervalSeconds: 1,
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
			k := &KeepalivedSpec{
				VRRPInstances: tt.vrrps,
			}
			err := k.validateVRRPInstances(returnNIC)
			if tt.wantErr {
				s.Require().NotEmpty(err, "Test case %s expected error. Got none", tt.name)
			} else {
				s.Require().Empty(err, "Test case %s expected no errors. Got: %v", tt.name, err)
				s.T().Log(k.VRRPInstances)
				s.Require().Equal(len(tt.expectedVRRPs), len(k.VRRPInstances), "Expected and actual VRRPInstances length mismatch")
				for i := 0; i < len(tt.expectedVRRPs); i++ {
					s.Require().Equal(tt.expectedVRRPs[i].Interface, k.VRRPInstances[i].Interface, "Interface mismatch")
					s.Require().Equal(tt.expectedVRRPs[i].VirtualRouterID, k.VRRPInstances[i].VirtualRouterID, "Virtual router ID mismatch")
					s.Require().Equal(tt.expectedVRRPs[i].AdvertIntervalSeconds, k.VRRPInstances[i].AdvertIntervalSeconds, "Advertisement interval mismatch")
				}
			}
		})
	}
}

func returnNIC() (string, error) {
	return "fake-nic-0", nil
}

func (s *CPLBSuite) TestValidateVirtualServers() {
	tests := []struct {
		name        string
		vss         []VirtualServer
		expectedVSS []VirtualServer
		wantErr     bool
	}{
		{
			name: "Set expected defaults",
			vss: []VirtualServer{
				{
					IPAddress: "1.2.3.4",
				},
				{
					IPAddress: "1.2.3.5",
				},
			},
			expectedVSS: []VirtualServer{
				{
					IPAddress:                 "1.2.3.4",
					DelayLoop:                 metav1.Duration{Duration: time.Minute},
					LBAlgo:                    RRAlgo,
					LBKind:                    DRLBKind,
					PersistenceTimeoutSeconds: 360,
				},
				{
					IPAddress:                 "1.2.3.5",
					DelayLoop:                 metav1.Duration{Duration: time.Minute},
					LBAlgo:                    RRAlgo,
					LBKind:                    DRLBKind,
					PersistenceTimeoutSeconds: 360,
				},
			},
			wantErr: false,
		},
		{
			name: "valid instance no overrides",
			vss: []VirtualServer{
				{
					IPAddress:                 "1.2.3.4",
					DelayLoop:                 metav1.Duration{Duration: 1 * time.Second},
					LBAlgo:                    WRRAlgo,
					LBKind:                    NATLBKind,
					PersistenceTimeoutSeconds: 100,
				},
			},
			expectedVSS: []VirtualServer{
				{
					IPAddress:                 "1.2.3.4",
					DelayLoop:                 metav1.Duration{Duration: 1 * time.Second},
					LBAlgo:                    WRRAlgo,
					LBKind:                    NATLBKind,
					PersistenceTimeoutSeconds: 100,
				},
			},
			wantErr: false,
		},
		{
			name: "truncate DelayLoop",
			vss: []VirtualServer{
				{
					IPAddress: "1.2.3.4",
					DelayLoop: metav1.Duration{Duration: 1234567 * time.Nanosecond},
				},
			},
			expectedVSS: []VirtualServer{
				{
					IPAddress:                 "1.2.3.4",
					DelayLoop:                 metav1.Duration{Duration: 1234 * time.Microsecond},
					LBAlgo:                    RRAlgo,
					LBKind:                    DRLBKind,
					PersistenceTimeoutSeconds: 360,
				},
			},
			wantErr: false,
		},
		{
			name:    "empty ip address",
			vss:     []VirtualServer{{}},
			wantErr: true,
		},
		{
			name: "invalid IP address",
			vss: []VirtualServer{{
				IPAddress: "INVALID",
			}},
			wantErr: true,
		},
		{
			name: "invalid LBAlgo",
			vss: []VirtualServer{{
				LBAlgo: "invalid",
			}},
			wantErr: true,
		},
		{
			name: "invalid LBKind",
			vss: []VirtualServer{{
				LBKind: "invalid",
			}},
			wantErr: true,
		},
		{
			name: "invalid persistencee timeout",
			vss: []VirtualServer{{
				PersistenceTimeoutSeconds: -1,
			}},
			wantErr: true,
		},
		{
			name: "invalid delay loop",
			vss: []VirtualServer{{
				DelayLoop: metav1.Duration{Duration: -1},
			}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			k := &KeepalivedSpec{VirtualServers: tt.vss}
			err := k.validateVirtualServers()
			if tt.wantErr {
				s.Require().NotEmpty(err, "Test case %s expected error. Got none", tt.name)
			} else {
				s.Require().Empty(err, "Tedst case %s expected no error. Got: %v", tt.name, err)
				for i := range tt.expectedVSS {
					s.Require().Equal(tt.expectedVSS[i].DelayLoop, k.VirtualServers[i].DelayLoop, "DelayLoop mismatch")
					s.Require().Equal(tt.expectedVSS[i].LBAlgo, k.VirtualServers[i].LBAlgo, "LBalgo mismatch")
					s.Require().Equal(tt.expectedVSS[i].LBKind, k.VirtualServers[i].LBKind, "LBKind mismatch")
					s.Require().Equal(tt.expectedVSS[i].PersistenceTimeoutSeconds, k.VirtualServers[i].PersistenceTimeoutSeconds, "PersistenceTimeout mismatch")
				}
			}
		})
	}
}
func TestCPLBSuite(t *testing.T) {
	cplbSuite := &CPLBSuite{}

	suite.Run(t, cplbSuite)
}
