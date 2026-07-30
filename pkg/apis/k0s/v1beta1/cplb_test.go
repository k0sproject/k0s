// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"math"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateVRRPInstances(t *testing.T) {
	tests := []struct {
		name          string
		vrrps         VRRPInstances
		expectedVRRPs VRRPInstances
		wantErr       string
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
		}, {
			name: "Lowest and highest virtual router IDs",
			vrrps: VRRPInstances{{
				VirtualRouterID: 1,
				Interface:       "eth0",
				VirtualIPs:      []string{"192.168.1.1/24"},
				AuthPass:        "123456",
			}, {
				VirtualRouterID: 255,
				Interface:       "eth0",
				VirtualIPs:      []string{"192.168.2.1/24"},
				AuthPass:        "123456",
			}},
			expectedVRRPs: VRRPInstances{{
				VirtualRouterID:       1,
				Interface:             "eth0",
				AdvertIntervalSeconds: defaultAdvertIntervalSeconds,
			}, {
				VirtualRouterID:       255,
				Interface:             "eth0",
				AdvertIntervalSeconds: defaultAdvertIntervalSeconds,
			}},
		}, {
			name: "Virtual router ID too high",
			vrrps: VRRPInstances{{
				Interface:  "eth0",
				VirtualIPs: []string{"192.168.1.1/24"},
				AuthPass:   "123456",
			}, {
				VirtualRouterID: 256,
				Interface:       "eth0",
				VirtualIPs:      []string{"192.168.2.1/24"},
				AuthPass:        "123456",
			}},
			wantErr: `vrrpInstances[1].virtualRouterID: Invalid value: 256: must be between 1 and 255, inclusive`,
		}, {
			name: "Negative virtual router ID",
			vrrps: VRRPInstances{{
				VirtualRouterID: -1,
				Interface:       "eth0",
				VirtualIPs:      []string{"192.168.1.1/24"},
				AuthPass:        "123456",
			}},
			wantErr: `vrrpInstances[0].virtualRouterID: Invalid value: -1: must be between 1 and 255, inclusive`,
		}, {
			name: "Reserved address label",
			vrrps: VRRPInstances{{
				Interface:    "eth0",
				VirtualIPs:   []string{"192.168.1.1/24"},
				AuthPass:     "123456",
				AddressLabel: math.MaxUint32,
			}},
			wantErr: `vrrpInstances[0].addressLabel: Invalid value: 4294967295: 0xffffffff is reserved`,
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
			wantErr: `vrrpInstances[0].authPass: Required value`,
		}, {
			name: "Password too long",
			vrrps: VRRPInstances{
				{
					VirtualIPs: []string{"192.168.1.1/24"},
					AuthPass:   "012345678",
				},
			},
			wantErr: `vrrpInstances[0].authPass: Too long: may not be more than 8 bytes`,
		}, {
			name: "No virtual IPs",
			vrrps: VRRPInstances{{
				Interface: "eth0",
				AuthPass:  "123456",
			}},
			wantErr: `vrrpInstances[0].virtualIPs: Required value`,
		}, {
			name: "Invalid CIDR",
			vrrps: VRRPInstances{
				{
					VirtualIPs: []string{"192.168.1.1/24", "192.168.1.1"},
					AuthPass:   "123456",
				},
			},
			wantErr: `vrrpInstances[0].virtualIPs[1]: Invalid value: "192.168.1.1": must be a valid address in CIDR form, (e.g. 10.9.8.7/24 or 2001:db8::1/64)`,
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
			wantErr: `vrrpInstances[0].unicastSourceIP: Required value: when unicastPeers are specified`,
		}, {
			name: "Invalid unicast peers",
			vrrps: VRRPInstances{
				{
					VirtualRouterID:       1,
					Interface:             "eth0",
					VirtualIPs:            []string{"192.168.1.100/24"},
					AdvertIntervalSeconds: 1,
					AuthPass:              "123456",
					UnicastSourceIP:       "192.168.1.1",
					UnicastPeers:          []string{"192.168.1.2", "example.com"},
				},
			},
			wantErr: `vrrpInstances[0].unicastPeers[1]: Invalid value: "example.com": must be a valid IP address, (e.g. 10.9.8.7 or 2001:db8::ffff)`,
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
			wantErr: `vrrpInstances[0].unicastSourceIP: Invalid value: "example.com": must be a valid IP address, (e.g. 10.9.8.7 or 2001:db8::ffff)`,
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
			wantErr: `vrrpInstances[0].unicastPeers[0]: Invalid value: "192.168.1.1": must not be the same as unicastSourceIP`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			underTest := tt.vrrps.DeepCopy()
			errs := underTest.validate(field.NewPath("vrrpInstances"), returnNIC)
			if tt.wantErr != "" {
				if assert.Len(t, errs, 1) {
					assert.ErrorContains(t, errs[0], tt.wantErr)
				}
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

	t.Run("automatic virtualRouterIDs exhausted", func(t *testing.T) {
		// The automatic IDs start at the default one, so this many instances
		// makes the last one exceed the maximum by one.
		var underTest [255 - defaultVirtualRouterID + 2]VRRPInstance
		for i := range underTest {
			underTest[i] = VRRPInstance{
				Interface:  "eth0",
				VirtualIPs: []string{"192.168.1.1/24"},
				AuthPass:   "123456",
			}
		}

		errs := VRRPInstances(underTest[:]).validate(field.NewPath("vrrpInstances"), returnNIC)

		if assert.Lenf(t, errs, 1, "Expected exactly one error") {
			assert.ErrorContains(t, errs[0],
				`vrrpInstances[205].virtualRouterID: Internal error: automatic virtualRouterIDs exceeded, specify them explicitly`,
			)
		}
		assert.EqualValues(t, 255, underTest[len(underTest)-2].VirtualRouterID)
		assert.Zero(t, underTest[len(underTest)-1].VirtualRouterID)
	})
}

func returnNIC() (string, error) {
	return "fake-nic-0", nil
}

func TestValidateVirtualServers(t *testing.T) {
	tests := []struct {
		name        string
		vss         VirtualServers
		expectedVSS VirtualServers
		wantErr     string
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
		},
		{
			name:    "empty ip address",
			vss:     VirtualServers{{IPAddress: "1.2.3.4"}, {}},
			wantErr: `virtualServers[1].ipAddress: Required value`,
		},
		{
			name: "invalid IP address",
			vss: VirtualServers{{
				IPAddress: "INVALID",
			}},
			wantErr: `virtualServers[0].ipAddress: Invalid value: "INVALID": must be a valid IP address, (e.g. 10.9.8.7 or 2001:db8::ffff)`,
		},
		{
			name: "invalid LBAlgo",
			vss: VirtualServers{{
				IPAddress: "1.2.3.4",
				LBAlgo:    "invalid",
			}},
			wantErr: `virtualServers[0].lbAlgo: Unsupported value: "invalid": supported values: "rr", "wrr", "lc", "wlc", "lblc", "dh", "sh", "sed", "nq"`,
		},
		{
			name: "invalid LBKind",
			vss: VirtualServers{{
				IPAddress: "1.2.3.4",
				LBKind:    "invalid",
			}},
			wantErr: `virtualServers[0].lbKind: Unsupported value: "invalid": supported values: "NAT", "DR", "TUN"`,
		},
		{
			name: "negative persistence timeout",
			vss: VirtualServers{{
				IPAddress:                 "1.2.3.4",
				PersistenceTimeoutSeconds: -1,
			}},
			wantErr: `virtualServers[0].persistenceTimeoutSeconds: Invalid value: -1: must be between 1 and 2678400, inclusive`,
		},
		{
			name: "persistence timeout too long",
			vss: VirtualServers{{
				IPAddress:                 "1.2.3.4",
				PersistenceTimeoutSeconds: 2678401,
			}},
			wantErr: `virtualServers[0].persistenceTimeoutSeconds: Invalid value: 2678401: must be between 1 and 2678400, inclusive`,
		},
		{
			name: "negative delay loop",
			vss: VirtualServers{{
				IPAddress: "1.2.3.4",
				DelayLoop: metav1.Duration{Duration: -1 * time.Second},
			}},
			wantErr: `virtualServers[0].delayLoop: Invalid value: "-1s": must be positive`,
		},
		{
			name: "sub-microsecond delay loop",
			vss: VirtualServers{{
				IPAddress: "1.2.3.4",
				DelayLoop: metav1.Duration{Duration: 999 * time.Nanosecond},
			}},
			wantErr: `virtualServers[0].delayLoop: Invalid value: "0s": must be positive`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			underTest := tt.vss.DeepCopy()
			errs := underTest.validate(field.NewPath("virtualServers"))
			if tt.wantErr != "" {
				if assert.Len(t, errs, 1) {
					assert.ErrorContains(t, errs[0], tt.wantErr)
				}
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

func TestKeepalivedSpec_Validate(t *testing.T) {
	// userSpaceProxyBindPort
	for _, tt := range []struct {
		name     string
		port     int
		wantPort int
		wantErr  string
	}{
		{
			name:     "defaults to 6444",
			wantPort: 6444,
		},
		{
			name:     "accepts custom port",
			port:     7000,
			wantPort: 7000,
		},
		{
			name:    "rejects out of range port",
			port:    70000,
			wantErr: "userSpaceProxyBindPort: Invalid value: 70000: must be between 1 and 65535, inclusive",
		},
	} {
		t.Run("userSpaceProxyBindPort "+tt.name, func(t *testing.T) {
			k := &KeepalivedSpec{UserSpaceProxyPort: tt.port}
			errs := k.Validate(nil)
			if tt.wantErr != "" {
				require.Len(t, errs, 1)
				assert.ErrorContains(t, errs[0], tt.wantErr)
			} else {
				require.Empty(t, errs)
				require.Equal(t, tt.wantPort, k.UserSpaceProxyPort)
			}
		})
	}
}
