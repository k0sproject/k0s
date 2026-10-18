// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cplb

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/netip"
	"sync"
	"syscall"
	"time"

	k0sAPI "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	"github.com/bombsimon/logrusr/v4"
	"github.com/go-logr/logr"
	api "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
	"github.com/osrg/gobgp/v4/pkg/server"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

// BGP advertises the virtual IPs as host routes to the BGP peers while the
// local API server is ready, leaving the load balancing to the peers.
type BGP struct {
	Config *k0sAPI.BGPSpec

	log    *logrus.Entry
	server *server.BgpServer
	lo     netlink.Link
	stop   func()

	KubeConfigPath string
	Address        string
}

func (b *BGP) Init(context.Context) error {
	b.log = logrus.WithField("component", "cplb-bgp")
	return nil
}

func (b *BGP) Start(ctx context.Context) error {
	routerID := cmp.Or(b.Config.RouterID, b.Address)
	if ip := net.ParseIP(routerID); ip == nil || ip.To4() == nil {
		return errors.New("BGP needs an IPv4 router ID, set bgp.routerID")
	}

	// Virtual IPs inside a connected subnet would be reachable over layer 2.
	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("failed to list interfaces: %w", err)
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		var addrs []net.Addr
		if addrs, err = iface.Addrs(); err != nil {
			return fmt.Errorf("failed to list addresses of %s: %w", iface.Name, err)
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.IsLinkLocalUnicast() {
				continue
			}
			for _, vip := range b.Config.VirtualIPs {
				if ip, _, parseErr := net.ParseCIDR(vip); parseErr == nil && ipNet.Contains(ip) {
					return fmt.Errorf("virtual IP %s is inside the subnet %s of %s, BGP needs virtual IPs outside of connected subnets", ip, ipNet, iface.Name)
				}
			}
		}
	}

	if b.lo, err = netlink.LinkByName("lo"); err != nil {
		return fmt.Errorf("failed to get lo: %w", err)
	}
	b.setAdvertised(false)

	client, err := kubeutil.NewClientFromFile(b.KubeConfigPath)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	if err = b.startServer(ctx, routerID); err != nil {
		b.server.Stop()
		b.server = nil
		return err
	}

	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	var wg sync.WaitGroup
	wg.Go(func() {
		advertiseWhileReady(runCtx, func(ctx context.Context) error {
			_, err := client.Discovery().RESTClient().Get().AbsPath("/readyz").DoRaw(ctx)
			return err
		}, b.setAdvertised)
	})
	b.stop = func() { cancel(); wg.Wait() }

	return nil
}

func (b *BGP) startServer(ctx context.Context, routerID string) (err error) {
	logger := slog.New(logr.ToSlogHandler(logrusr.New(b.log)))
	b.server = server.NewBgpServer(server.LoggerOption(logger, &slog.LevelVar{}))
	go b.server.Serve()

	if err = b.server.StartBgp(ctx, &api.StartBgpRequest{Global: &api.Global{
		Asn:        b.Config.AS,
		RouterId:   routerID,
		ListenPort: -1,
	}}); err != nil {
		return fmt.Errorf("failed to start BGP: %w", err)
	}

	// The controllers only advertise their own routes, they never use received ones.
	localOnly := &api.Policy{Name: "k0s-cplb-local-only", Statements: []*api.Statement{{
		Name:       "k0s-cplb-local",
		Conditions: &api.Conditions{RouteType: api.Conditions_ROUTE_TYPE_LOCAL},
		Actions:    &api.Actions{RouteAction: api.RouteAction_ROUTE_ACTION_ACCEPT},
	}}}
	if err = b.server.AddPolicy(ctx, &api.AddPolicyRequest{Policy: localOnly}); err == nil {
		err = b.server.SetPolicyAssignment(ctx, &api.SetPolicyAssignmentRequest{Assignment: &api.PolicyAssignment{
			Direction:     api.PolicyDirection_POLICY_DIRECTION_IMPORT,
			Policies:      []*api.Policy{{Name: localOnly.Name}},
			DefaultAction: api.RouteAction_ROUTE_ACTION_REJECT,
		}})
	}
	if err != nil {
		return fmt.Errorf("failed to reject received routes: %w", err)
	}

	for _, p := range b.Config.Peers {
		peer := &api.Peer{
			Conf: &api.PeerConf{NeighborAddress: p.Address, PeerAsn: p.AS, AuthPassword: p.Password},
			Timers: &api.Timers{Config: &api.TimersConfig{
				ConnectRetry:      10,
				HoldTime:          uint64(b.Config.HoldTimeSeconds),
				KeepaliveInterval: uint64(b.Config.KeepaliveIntervalSeconds),
			}},
			Transport: &api.Transport{
				RemoteAddress: p.Address,
				RemotePort:    p.Port,
				LocalAddress:  b.Config.SourceIP,
				BindInterface: b.Config.SourceInterface,
			},
		}
		if p.Multihop {
			peer.EbgpMultihop = &api.EbgpMultihop{Enabled: true, MultihopTtl: 255}
		}
		if p.BFD != nil {
			peer.Bfd = &api.BfdPeerConfig{
				Enabled:                  true,
				Port:                     server.BfdServerPort,
				DesiredMinimumTxInterval: p.BFD.MinimumTransmitMilliseconds * 1000,
				RequiredMinimumReceive:   p.BFD.MinimumReceiveMilliseconds * 1000,
				DetectionMultiplier:      p.BFD.Multiplier,
			}
		}
		if err = b.server.AddPeer(ctx, &api.AddPeerRequest{Peer: peer}); err != nil {
			return fmt.Errorf("failed to add BGP peer %s: %w", p.Address, err)
		}
	}

	return nil
}

// Stop withdraws the routes, and stopping the server tells the peers to drop them right away.
func (b *BGP) Stop() error {
	if b.stop != nil {
		b.stop()
	}
	if b.server != nil {
		b.server.Stop()
		b.server = nil
	}
	return nil
}

// advertiseWhileReady advertises while the ready check succeeds, tolerating a
// few failed checks so that a single blip doesn't move the routes.
func advertiseWhileReady(ctx context.Context, check func(context.Context) error, set func(bool)) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	ready, failures := false, 0
	for {
		select {
		case <-ctx.Done():
			if ready {
				set(false)
			}
			return
		case <-ticker.C:
		}

		checkCtx, cancel := context.WithTimeout(ctx, time.Second)
		err := check(checkCtx)
		cancel()
		switch {
		case err == nil:
			failures = 0
			if !ready {
				ready = true
				set(true)
			}
		case ready:
			if failures++; failures >= 3 {
				logrus.WithField("component", "cplb-bgp").WithError(err).Warn("The local API server isn't ready, withdrawing the virtual IPs")
				ready = false
				set(false)
			}
		}
	}
}

func (b *BGP) setAdvertised(advertised bool) {
	for _, vip := range b.Config.VirtualIPs {
		prefix, err := netip.ParsePrefix(vip)
		if err != nil {
			b.log.WithError(err).Errorf("Invalid virtual IP %s", vip)
			continue
		}
		// A preferred lifetime of zero marks the address deprecated, which keeps it from being
		// picked as the node address, or as a source address for IPv6.
		addr := &netlink.Addr{IPNet: &net.IPNet{IP: prefix.Addr().AsSlice(), Mask: net.CIDRMask(prefix.Bits(), prefix.Addr().BitLen())}, ValidLft: math.MaxInt}
		path, err := hostRoute(prefix.Addr())
		if err != nil {
			b.log.WithError(err).Errorf("Failed to build the route of %s", vip)
			continue
		}

		if advertised {
			if err = netlink.AddrReplace(b.lo, addr); err == nil {
				_, err = b.server.AddPath(apiutil.AddPathRequest{Paths: []*apiutil.Path{path}})
			}
		} else {
			if b.server != nil {
				err = b.server.DeletePath(apiutil.DeletePathRequest{Paths: []*apiutil.Path{path}})
			}
			if delErr := netlink.AddrDel(b.lo, addr); delErr != nil && !errors.Is(delErr, syscall.EADDRNOTAVAIL) {
				err = errors.Join(err, delErr)
			}
		}
		if err != nil {
			b.log.WithError(err).Errorf("Failed to update the advertisement of %s", vip)
		}
	}
}

// hostRoute leaves the next hop unspecified, so that gobgp sets it to the session address.
func hostRoute(addr netip.Addr) (*apiutil.Path, error) {
	prefix, err := bgp.NewIPAddrPrefix(netip.PrefixFrom(addr, addr.BitLen()))
	if err != nil {
		return nil, err
	}

	if addr.Is4() {
		var nextHop *bgp.PathAttributeNextHop
		if nextHop, err = bgp.NewPathAttributeNextHop(netip.IPv4Unspecified()); err != nil {
			return nil, err
		}
		return &apiutil.Path{
			Family: bgp.RF_IPv4_UC,
			Nlri:   prefix,
			Attrs:  []bgp.PathAttributeInterface{bgp.NewPathAttributeOrigin(0), nextHop},
		}, nil
	}

	reach, err := bgp.NewPathAttributeMpReachNLRI(bgp.RF_IPv6_UC, []bgp.PathNLRI{{NLRI: prefix}}, netip.IPv6Unspecified())
	if err != nil {
		return nil, err
	}
	return &apiutil.Path{
		Family: bgp.RF_IPv6_UC,
		Nlri:   prefix,
		Attrs:  []bgp.PathAttributeInterface{bgp.NewPathAttributeOrigin(0), reach},
	}, nil
}
