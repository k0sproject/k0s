// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cplb

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	k0sAPI "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	api "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
	"github.com/osrg/gobgp/v4/pkg/server"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHostRoute(t *testing.T) {
	path, err := hostRoute(netip.MustParseAddr("203.0.113.10"))
	require.NoError(t, err)
	assert.Equal(t, bgp.RF_IPv4_UC, path.Family)
	assert.Equal(t, "203.0.113.10/32", path.Nlri.String())

	path, err = hostRoute(netip.MustParseAddr("2001:db8::10"))
	require.NoError(t, err)
	assert.Equal(t, bgp.RF_IPv6_UC, path.Family)
	assert.Equal(t, "2001:db8::10/128", path.Nlri.String())
}

func TestBGPServer(t *testing.T) {
	for _, test := range []struct {
		name string
		as   uint32
	}{{"eBGP", 65001}, {"iBGP", 65000}} {
		t.Run(test.name, func(t *testing.T) {
			listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
			require.NoError(t, err)
			addr, err := netip.ParseAddrPort(listener.Addr().String())
			require.NoError(t, err)
			port := addr.Port()
			require.NoError(t, listener.Close())

			peer := server.NewBgpServer()
			go peer.Serve()
			t.Cleanup(peer.Stop)
			require.NoError(t, peer.StartBgp(t.Context(), &api.StartBgpRequest{Global: &api.Global{
				Asn: 65000, RouterId: "127.0.0.1", ListenPort: int32(port), ListenAddresses: []string{"127.0.0.1"},
			}}))
			require.NoError(t, peer.AddPeer(t.Context(), &api.AddPeerRequest{Peer: &api.Peer{
				Conf:      &api.PeerConf{NeighborAddress: "127.0.0.2", PeerAsn: test.as},
				Transport: &api.Transport{PassiveMode: true},
			}}))

			b := &BGP{log: logrus.WithField("test", t.Name()), Config: &k0sAPI.BGPSpec{
				AS: test.as, SourceIP: "127.0.0.2", HoldTimeSeconds: 9, KeepaliveIntervalSeconds: 3,
				Peers: []k0sAPI.BGPPeer{{Address: "127.0.0.1", AS: 65000, Port: uint32(port)}},
			}}
			require.NoError(t, b.startServer(t.Context(), "127.0.0.2"))
			t.Cleanup(b.server.Stop)

			nextHops := func() []netip.Addr {
				var hops []netip.Addr
				assert.NoError(t, peer.ListPath(apiutil.ListPathRequest{TableType: api.TableType_TABLE_TYPE_GLOBAL, Family: bgp.RF_IPv4_UC},
					func(prefix bgp.NLRI, paths []*apiutil.Path) {
						if prefix.String() != "203.0.113.10/32" {
							return
						}
						for _, p := range paths {
							for _, attr := range p.Attrs {
								if nextHop, ok := attr.(*bgp.PathAttributeNextHop); ok {
									hops = append(hops, nextHop.Value)
								}
							}
						}
					}))
				return hops
			}

			path, err := hostRoute(netip.MustParseAddr("203.0.113.10"))
			require.NoError(t, err)
			_, err = b.server.AddPath(apiutil.AddPathRequest{Paths: []*apiutil.Path{path}})
			require.NoError(t, err)
			assert.Eventually(t, func() bool {
				hops := nextHops()
				return len(hops) == 1 && hops[0] == netip.MustParseAddr("127.0.0.2")
			}, time.Minute, 100*time.Millisecond, "the peer should receive the host route with the controller as next hop")

			require.NoError(t, b.server.DeletePath(apiutil.DeletePathRequest{Paths: []*apiutil.Path{path}}))
			assert.Eventually(t, func() bool { return len(nextHops()) == 0 }, 10*time.Second, 100*time.Millisecond,
				"the peer should drop the withdrawn route")

			received, err := hostRoute(netip.MustParseAddr("198.51.100.10"))
			require.NoError(t, err)
			received.Attrs[1], err = bgp.NewPathAttributeNextHop(netip.MustParseAddr("127.0.0.1"))
			require.NoError(t, err)
			_, err = peer.AddPath(apiutil.AddPathRequest{Paths: []*apiutil.Path{received}})
			require.NoError(t, err)
			hasRoute := func(table api.TableType, name string) func() bool {
				return func() bool {
					var found bool
					assert.NoError(t, b.server.ListPath(apiutil.ListPathRequest{TableType: table, Name: name, Family: bgp.RF_IPv4_UC},
						func(prefix bgp.NLRI, _ []*apiutil.Path) { found = found || prefix.String() == "198.51.100.10/32" }))
					return found
				}
			}
			assert.Eventually(t, hasRoute(api.TableType_TABLE_TYPE_ADJ_IN, "127.0.0.1"), 10*time.Second, 100*time.Millisecond,
				"the controller should receive the peer's route")
			assert.Never(t, hasRoute(api.TableType_TABLE_TYPE_GLOBAL, ""), 3*time.Second, 100*time.Millisecond,
				"routes received from peers should be rejected")
		})
	}
}

func TestAdvertiseWhileReady(t *testing.T) {
	var ready, advertised atomic.Bool
	var mu sync.Mutex
	var calls []bool
	set := func(v bool) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, v)
		advertised.Store(v)
	}
	check := func(context.Context) error {
		if ready.Load() {
			return nil
		}
		return errors.New("not ready")
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); advertiseWhileReady(ctx, check, set) }()

	time.Sleep(1500 * time.Millisecond)
	assert.False(t, advertised.Load(), "nothing should be advertised before the API server is ready")

	ready.Store(true)
	assert.Eventually(t, advertised.Load, 5*time.Second, 100*time.Millisecond,
		"the virtual IPs should be advertised once the API server is ready")

	ready.Store(false)
	time.Sleep(1500 * time.Millisecond)
	assert.True(t, advertised.Load(), "a single failed check shouldn't withdraw the virtual IPs")
	assert.Eventually(t, func() bool { return !advertised.Load() }, 5*time.Second, 100*time.Millisecond,
		"the virtual IPs should be withdrawn after repeated failed checks")

	ready.Store(true)
	assert.Eventually(t, advertised.Load, 5*time.Second, 100*time.Millisecond)
	cancel()
	<-done
	assert.False(t, advertised.Load(), "stopping should withdraw the virtual IPs")
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []bool{true, false, true, false}, calls)
}
