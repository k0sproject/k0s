// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/require"
)

func TestValidateVRRPInstances(t *testing.T) {
	tests := []struct {
		name          string
		vrrps         VRRPInstances
		expectedVRRPs VRRPInstances
		wantErr       bool
	}{
		{
			name: "Set expected defaults",
			vrrps: VRRPInstances{
				{
					VirtualIPs: []string{"192.168.1.1/24"},
					AuthPass:   "123456",
				},
				{
					VirtualIPs: []string{"192.168.7.1/24"},
					AuthPass:   "12345678",
				},
			},
			expectedVRRPs: VRRPInstances{
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
			vrrps: VRRPInstances{
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
			expectedVRRPs: VRRPInstances{
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
			vrrps: VRRPInstances{
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
			vrrps: VRRPInstances{
				{
					VirtualIPs: []string{"192.168.1.1/24"},
					AuthPass:   "012345678",
				},
			},
			wantErr: true,
		}, {
			name: "Invalid CIDR",
			vrrps: VRRPInstances{
				{
					VirtualIPs: []string{"192.168.1.1"},
					AuthPass:   "123456",
				},
			},
			wantErr: true,
		}, {
			name: "Unicast Peers without unicast source",
			vrrps: VRRPInstances{
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
			vrrps: VRRPInstances{
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
			vrrps: VRRPInstances{
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
			vrrps: VRRPInstances{
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
		t.Run(tt.name, func(t *testing.T) {
			underTest := tt.vrrps.DeepCopy()
			errs := underTest.validate(returnNIC)
			if tt.wantErr {
				require.Error(t, errors.Join(errs...))
			} else {
				require.Empty(t, errs)
				t.Log(underTest)
				require.Len(t, underTest, len(tt.expectedVRRPs), "Expected and actual VRRPInstances length mismatch")
				for i := range tt.expectedVRRPs {
					require.Equal(t, tt.expectedVRRPs[i].Interface, underTest[i].Interface, "Interface mismatch")
					require.Equal(t, tt.expectedVRRPs[i].VirtualRouterID, underTest[i].VirtualRouterID, "Virtual router ID mismatch")
					require.Equal(t, tt.expectedVRRPs[i].AdvertIntervalSeconds, underTest[i].AdvertIntervalSeconds, "Advertisement interval mismatch")
				}
			}
		})
	}
}

func returnNIC() (string, error) {
	return "fake-nic-0", nil
}

func TestValidateVirtualServers(t *testing.T) {
	tests := []struct {
		name        string
		vss         VirtualServers
		expectedVSS VirtualServers
		wantErr     bool
	}{
		{
			name: "Set expected defaults",
			vss: VirtualServers{
				{
					IPAddress: "1.2.3.4",
				},
				{
					IPAddress: "1.2.3.5",
				},
			},
			expectedVSS: VirtualServers{
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
			vss: VirtualServers{
				{
					IPAddress:                 "1.2.3.4",
					DelayLoop:                 metav1.Duration{Duration: 1 * time.Second},
					LBAlgo:                    WRRAlgo,
					LBKind:                    NATLBKind,
					PersistenceTimeoutSeconds: 100,
				},
			},
			expectedVSS: VirtualServers{
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
			vss: VirtualServers{
				{
					IPAddress: "1.2.3.4",
					DelayLoop: metav1.Duration{Duration: 1234567 * time.Nanosecond},
				},
			},
			expectedVSS: VirtualServers{
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
			vss:     VirtualServers{{}},
			wantErr: true,
		},
		{
			name: "invalid IP address",
			vss: VirtualServers{{
				IPAddress: "INVALID",
			}},
			wantErr: true,
		},
		{
			name: "invalid LBAlgo",
			vss: VirtualServers{{
				LBAlgo: "invalid",
			}},
			wantErr: true,
		},
		{
			name: "invalid LBKind",
			vss: VirtualServers{{
				LBKind: "invalid",
			}},
			wantErr: true,
		},
		{
			name: "invalid persistencee timeout",
			vss: VirtualServers{{
				PersistenceTimeoutSeconds: -1,
			}},
			wantErr: true,
		},
		{
			name: "invalid delay loop",
			vss: VirtualServers{{
				DelayLoop: metav1.Duration{Duration: -1},
			}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			underTest := tt.vss.DeepCopy()
			errs := underTest.validate()
			if tt.wantErr {
				require.Error(t, errors.Join(errs...))
			} else {
				require.Empty(t, errs)
				for i := range tt.expectedVSS {
					require.Equal(t, tt.expectedVSS[i].DelayLoop, underTest[i].DelayLoop, "DelayLoop mismatch")
					require.Equal(t, tt.expectedVSS[i].LBAlgo, underTest[i].LBAlgo, "LBalgo mismatch")
					require.Equal(t, tt.expectedVSS[i].LBKind, underTest[i].LBKind, "LBKind mismatch")
					require.Equal(t, tt.expectedVSS[i].PersistenceTimeoutSeconds, underTest[i].PersistenceTimeoutSeconds, "PersistenceTimeout mismatch")
				}
			}
		})
	}
}
