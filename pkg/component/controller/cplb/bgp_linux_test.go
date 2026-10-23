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
	path, err := hostRoute(netip.MustParseAddr("203.0.113.10"), nil)
	require.NoError(t, err)
	assert.Equal(t, bgp.RF_IPv4_UC, path.Family)
	assert.Equal(t, "203.0.113.10/32", path.Nlri.String())

	path, err = hostRoute(netip.MustParseAddr("2001:db8::10"), nil)
	require.NoError(t, err)
	assert.Equal(t, bgp.RF_IPv6_UC, path.Family)
	assert.Equal(t, "2001:db8::10/128", path.Nlri.String())

	path, err = hostRoute(netip.MustParseAddr("203.0.113.10"), []string{"65000:100", "no-export", "65000:1:2"})
	require.NoError(t, err)
	assert.Contains(t, path.Attrs, bgp.NewPathAttributeCommunities([]uint32{65000<<16 | 100, uint32(bgp.COMMUNITY_NO_EXPORT)}))
	assert.Contains(t, path.Attrs, bgp.NewPathAttributeLargeCommunities([]*bgp.LargeCommunity{bgp.NewLargeCommunity(65000, 1, 2)}))

	_, err = hostRoute(netip.MustParseAddr("203.0.113.10"), []string{"65536:1"})
	assert.ErrorContains(t, err, "invalid community")
}

func TestBGPServer(t *testing.T) {
	for _, test := range []struct {
		name string
		as   uint32
	}{{"eBGP", 65001}, {"iBGP", 65000}} {
		t.Run(test.name, func(t *testing.T) {
			port := freePort(t, "127.0.0.1")

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

			path, err := hostRoute(netip.MustParseAddr("203.0.113.10"), []string{"65000:100", "65000:1:2"})
			require.NoError(t, err)
			_, err = b.server.AddPath(apiutil.AddPathRequest{Paths: []*apiutil.Path{path}})
			require.NoError(t, err)
			assert.Eventually(t, func() bool {
				hops := nextHops()
				return len(hops) == 1 && hops[0] == netip.MustParseAddr("127.0.0.2")
			}, time.Minute, 100*time.Millisecond, "the peer should receive the host route with the controller as next hop")
			var communities []bgp.PathAttributeInterface
			assert.NoError(t, peer.ListPath(apiutil.ListPathRequest{TableType: api.TableType_TABLE_TYPE_GLOBAL, Family: bgp.RF_IPv4_UC},
				func(_ bgp.NLRI, paths []*apiutil.Path) {
					for _, attr := range paths[0].Attrs {
						if attr.GetType() == bgp.BGP_ATTR_TYPE_COMMUNITIES || attr.GetType() == bgp.BGP_ATTR_TYPE_LARGE_COMMUNITY {
							communities = append(communities, attr)
						}
					}
				}))
			assert.Len(t, communities, 2, "the peer should receive the standard and large communities")

			require.NoError(t, b.server.DeletePath(apiutil.DeletePathRequest{Paths: []*apiutil.Path{path}}))
			assert.Eventually(t, func() bool { return len(nextHops()) == 0 }, 10*time.Second, 100*time.Millisecond,
				"the peer should drop the withdrawn route")

			received, err := hostRoute(netip.MustParseAddr("198.51.100.10"), nil)
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

func TestBGPServerExtendedNextHop(t *testing.T) {
	port := freePort(t, "::1")

	peer := server.NewBgpServer()
	go peer.Serve()
	t.Cleanup(peer.Stop)
	require.NoError(t, peer.StartBgp(t.Context(), &api.StartBgpRequest{Global: &api.Global{
		Asn: 65000, RouterId: "127.0.0.1", ListenPort: int32(port), ListenAddresses: []string{"::1"},
	}}))
	require.NoError(t, peer.AddPeer(t.Context(), &api.AddPeerRequest{Peer: &api.Peer{
		Conf:      &api.PeerConf{NeighborAddress: "::1", PeerAsn: 65001},
		Transport: &api.Transport{PassiveMode: true},
		AfiSafis: []*api.AfiSafi{
			{Config: &api.AfiSafiConfig{Family: &api.Family{Afi: api.Family_AFI_IP6, Safi: api.Family_SAFI_UNICAST}, Enabled: true}},
			{Config: &api.AfiSafiConfig{Family: &api.Family{Afi: api.Family_AFI_IP, Safi: api.Family_SAFI_UNICAST}, Enabled: true}},
		},
	}}))

	b := &BGP{log: logrus.WithField("test", t.Name()), Config: &k0sAPI.BGPSpec{
		AS: 65001, HoldTimeSeconds: 9, KeepaliveIntervalSeconds: 3,
		Peers: []k0sAPI.BGPPeer{{Address: "::1", AS: 65000, Port: uint32(port), ExtendedNextHop: true}},
	}}
	require.NoError(t, b.startServer(t.Context(), "127.0.0.2"))
	t.Cleanup(b.server.Stop)

	path, err := hostRoute(netip.MustParseAddr("203.0.113.10"), nil)
	require.NoError(t, err)
	_, err = b.server.AddPath(apiutil.AddPathRequest{Paths: []*apiutil.Path{path}})
	require.NoError(t, err)
	assert.Eventually(t, func() bool {
		var hop netip.Addr
		assert.NoError(t, peer.ListPath(apiutil.ListPathRequest{TableType: api.TableType_TABLE_TYPE_GLOBAL, Family: bgp.RF_IPv4_UC},
			func(_ bgp.NLRI, paths []*apiutil.Path) {
				for _, attr := range paths[0].Attrs {
					if reach, ok := attr.(*bgp.PathAttributeMpReachNLRI); ok {
						hop = reach.Nexthop
					}
				}
			}))
		return hop == netip.MustParseAddr("::1")
	}, time.Minute, 100*time.Millisecond, "the IPv6 peer should receive the IPv4 route with an IPv6 next hop")
}

func TestBGPListen(t *testing.T) {
	for _, test := range []struct {
		name, source, password string
		accepted               bool
	}{
		{"inside the prefix", "127.0.0.2", "secret", true},
		{"wrong password", "127.0.0.2", "wrong", false},
		{"outside the prefix", "127.0.0.3", "secret", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			port := freePort(t, "127.0.0.1")
			b := &BGP{log: logrus.WithField("test", t.Name()), Config: &k0sAPI.BGPSpec{
				AS: 65001, HoldTimeSeconds: 9, KeepaliveIntervalSeconds: 3,
				Listen: &k0sAPI.BGPListen{Port: port, Addresses: []string{"127.0.0.1"}, Neighbors: []k0sAPI.BGPNeighbor{
					{Prefix: "127.0.0.2/32", AS: 65000, Password: "secret"},
				}},
			}}
			require.NoError(t, b.startServer(t.Context(), "127.0.0.1"))
			t.Cleanup(b.server.Stop)
			path, err := hostRoute(netip.MustParseAddr("203.0.113.10"), nil)
			require.NoError(t, err)
			_, err = b.server.AddPath(apiutil.AddPathRequest{Paths: []*apiutil.Path{path}})
			require.NoError(t, err)

			peer := server.NewBgpServer()
			go peer.Serve()
			t.Cleanup(peer.Stop)
			require.NoError(t, peer.StartBgp(t.Context(), &api.StartBgpRequest{Global: &api.Global{Asn: 65000, RouterId: test.source, ListenPort: -1}}))
			require.NoError(t, peer.AddPeer(t.Context(), &api.AddPeerRequest{Peer: &api.Peer{
				Conf:      &api.PeerConf{NeighborAddress: "127.0.0.1", PeerAsn: 65001, AuthPassword: test.password},
				Timers:    &api.Timers{Config: &api.TimersConfig{ConnectRetry: 1}},
				Transport: &api.Transport{RemotePort: uint32(port), LocalAddress: test.source},
			}}))

			received := func() bool {
				var found bool
				assert.NoError(t, peer.ListPath(apiutil.ListPathRequest{TableType: api.TableType_TABLE_TYPE_GLOBAL, Family: bgp.RF_IPv4_UC},
					func(prefix bgp.NLRI, _ []*apiutil.Path) { found = found || prefix.String() == "203.0.113.10/32" }))
				return found
			}
			if test.accepted {
				assert.Eventually(t, received, time.Minute, 100*time.Millisecond, "the neighbor should receive the host route")
			} else {
				assert.Never(t, received, 5*time.Second, 100*time.Millisecond, "the neighbor shouldn't get a session")
			}
		})
	}
}

func freePort(t *testing.T, address string) uint16 {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", net.JoinHostPort(address, "0"))
	if err != nil {
		t.Skip("can't listen on", address, err)
	}
	addr, err := netip.ParseAddrPort(listener.Addr().String())
	require.NoError(t, err)
	require.NoError(t, listener.Close())
	return addr.Port()
}

func TestAdvertiseWhileReady(t *testing.T) {
	var ready, advertised atomic.Bool
	var mu sync.Mutex
	var calls []bool
	set := func(v bool) error {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, v)
		advertised.Store(v)
		return nil
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
	assert.Equal(t, []bool{false, true, false, true, false}, calls)
}

func TestAdvertiseWhileReady_RetriesFailedUpdates(t *testing.T) {
	var ready, advertised atomic.Bool
	var failing atomic.Int32
	var mu sync.Mutex
	var calls []bool
	set := func(v bool) error {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, v)
		if failing.Load() > 0 {
			failing.Add(-1)
			return errors.New("injected")
		}
		advertised.Store(v)
		return nil
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

	failing.Store(2)
	ready.Store(true)
	assert.Eventually(t, advertised.Load, 10*time.Second, 100*time.Millisecond,
		"a failed advertisement should be retried")

	failing.Store(2)
	ready.Store(false)
	assert.Eventually(t, func() bool { return !advertised.Load() }, 10*time.Second, 100*time.Millisecond,
		"a failed withdrawal should be retried")

	failing.Store(1)
	ready.Store(true)
	assert.Eventually(t, func() bool { return failing.Load() == 0 }, 5*time.Second, 100*time.Millisecond)
	cancel()
	<-done
	mu.Lock()
	defer mu.Unlock()
	assert.False(t, calls[len(calls)-1], "stopping should withdraw the virtual IPs that failed to be advertised")
}
