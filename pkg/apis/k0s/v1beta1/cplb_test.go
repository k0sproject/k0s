// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"errors"
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
					VirtualIPs:            []string{"192.168.1.100/24"},
					AdvertIntervalSeconds: 1,
					AuthPass:              "123456",
					UnicastSourceIP:       "192.168.1.1",
					UnicastPeers:          []string{"192.168.1.2", "192.168.1.3"},
				},
			},
			expectedVRRPs: []VRRPInstance{
				{
					VirtualRouterID:       1,
					Interface:             "eth0",
					VirtualIPs:            []string{"192.168.1.1/24"},
					AdvertIntervalSeconds: 1,
					AuthPass:              "123456",
					UnicastSourceIP:       "192.168.1.1",
					UnicastPeers:          []string{"192.168.1.2", "192.168.1.3"},
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
		}, {
			name: "Unicast Peers without unicast source",
			vrrps: []VRRPInstance{
				{
					VirtualRouterID:       1,
					Interface:             "eth0",
					VirtualIPs:            []string{"192.168.1.100/24"},
					AdvertIntervalSeconds: 1,
					AuthPass:              "123456",
					UnicastPeers:          []string{"192.168.1.2", "192.168.1.3"},
				},
			},
			wantErr: true,
		}, {
			name: "Invalid unicast peers",
			vrrps: []VRRPInstance{
				{
					VirtualRouterID:       1,
					Interface:             "eth0",
					VirtualIPs:            []string{"192.168.1.100/24"},
					AdvertIntervalSeconds: 1,
					AuthPass:              "123456",
					UnicastPeers:          []string{"example.com", "192.168.1.3"},
				},
			},
			wantErr: true,
		}, {
			name: "Invalid unicast source",
			vrrps: []VRRPInstance{
				{
					VirtualRouterID:       1,
					Interface:             "eth0",
					VirtualIPs:            []string{"192.168.1.100/24"},
					AdvertIntervalSeconds: 1,
					AuthPass:              "123456",
					UnicastSourceIP:       "example.com",
					UnicastPeers:          []string{"192.168.1.2", "192.168.1.3"},
				},
			},
			wantErr: true,
		}, {
			name: "Unicast peers includes unicast source",
			vrrps: []VRRPInstance{
				{
					VirtualRouterID:       1,
					Interface:             "eth0",
					VirtualIPs:            []string{"192.168.1.100/24"},
					AdvertIntervalSeconds: 1,
					AuthPass:              "123456",
					UnicastSourceIP:       "192.168.1.1",
					UnicastPeers:          []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"},
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
			errs := k.validateVRRPInstances(returnNIC)
			if tt.wantErr {
				s.Require().Error(errors.Join(errs...))
			} else {
				s.Require().Empty(errs)
				s.T().Log(k.VRRPInstances)
				s.Require().Len(k.VRRPInstances, len(tt.expectedVRRPs), "Expected and actual VRRPInstances length mismatch")
				for i := range tt.expectedVRRPs {
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
			errs := k.validateVirtualServers()
			if tt.wantErr {
				s.Require().Error(errors.Join(errs...))
			} else {
				s.Require().Empty(errs)
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
