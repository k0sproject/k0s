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
	"strconv"
	"strings"
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
	"golang.org/x/sys/unix"
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

	client, err := kubeutil.NewClientFromFile(b.KubeConfigPath)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	if b.lo, err = netlink.LinkByName("lo"); err != nil {
		return fmt.Errorf("failed to get lo: %w", err)
	}

	if err = b.startServer(ctx, routerID); err != nil {
		_ = b.setAdvertised(false)
		b.server.Stop()
		b.server = nil
		return err
	}

	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	var wg sync.WaitGroup
	wg.Go(func() {
		advertiseWhileReady(runCtx, func(ctx context.Context) error {
			checkCtx, cancelCheck := context.WithTimeout(ctx, time.Second)
			defer cancelCheck()
			_, err := client.Discovery().RESTClient().Get().AbsPath("/readyz").DoRaw(checkCtx)
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

	global := &api.Global{Asn: b.Config.AS, RouterId: routerID, ListenPort: -1}
	if listen := b.Config.Listen; listen != nil {
		// gobgp only logs a failure to set the password of a prefix, which would accept unauthenticated sessions.
		for _, n := range listen.Neighbors {
			if err = probeTCPMD5(netip.MustParsePrefix(n.Prefix), n.Password); err != nil {
				return fmt.Errorf("TCP MD5 passwords for prefixes aren't supported: %w", err)
			}
		}
		global.ListenPort, global.ListenAddresses = int32(listen.Port), listen.Addresses
	}
	if err = b.server.StartBgp(ctx, &api.StartBgpRequest{Global: global}); err != nil {
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

	timers := func() *api.Timers {
		return &api.Timers{Config: &api.TimersConfig{
			ConnectRetry:      10,
			HoldTime:          uint64(b.Config.HoldTimeSeconds),
			KeepaliveInterval: uint64(b.Config.KeepaliveIntervalSeconds),
		}}
	}

	for _, p := range b.Config.Peers {
		peer := &api.Peer{
			Conf:   &api.PeerConf{NeighborAddress: p.Address, PeerAsn: p.AS, AuthPassword: p.Password},
			Timers: timers(),
			Transport: &api.Transport{
				RemoteAddress: p.Address,
				RemotePort:    p.Port,
				LocalAddress:  b.Config.SourceIP,
				BindInterface: b.Config.SourceInterface,
			},
		}
		peer.EbgpMultihop, peer.AfiSafis, peer.Bfd = sessionOptions(p.Multihop, p.ExtendedNextHop, p.BFD)
		if err = b.server.AddPeer(ctx, &api.AddPeerRequest{Peer: peer}); err != nil {
			return fmt.Errorf("failed to add BGP peer %s: %w", p.Address, err)
		}
	}

	if listen := b.Config.Listen; listen != nil {
		for i, n := range listen.Neighbors {
			group := &api.PeerGroup{
				Conf:   &api.PeerGroupConf{PeerGroupName: fmt.Sprintf("k0s-cplb-neighbors-%d", i), PeerAsn: n.AS, AuthPassword: n.Password},
				Timers: timers(),
			}
			group.EbgpMultihop, group.AfiSafis, group.Bfd = sessionOptions(n.Multihop, n.ExtendedNextHop, n.BFD)
			if err = b.server.AddPeerGroup(ctx, &api.AddPeerGroupRequest{PeerGroup: group}); err == nil {
				err = b.server.AddDynamicNeighbor(ctx, &api.AddDynamicNeighborRequest{DynamicNeighbor: &api.DynamicNeighbor{
					Prefix: n.Prefix, PeerGroup: group.Conf.PeerGroupName,
				}})
			}
			if err != nil {
				return fmt.Errorf("failed to accept BGP neighbors from %s: %w", n.Prefix, err)
			}
		}
	}

	return nil
}

// sessionOptions maps the options that peers and listen neighbors share.
func sessionOptions(multihop, extendedNextHop bool, bfd *k0sAPI.BFDSpec) (ebgpMultihop *api.EbgpMultihop, families []*api.AfiSafi, bfdConfig *api.BfdPeerConfig) {
	if multihop {
		ebgpMultihop = &api.EbgpMultihop{Enabled: true, MultihopTtl: 255}
	}
	if extendedNextHop {
		for _, afi := range []api.Family_Afi{api.Family_AFI_IP6, api.Family_AFI_IP} {
			families = append(families, &api.AfiSafi{Config: &api.AfiSafiConfig{
				Family: &api.Family{Afi: afi, Safi: api.Family_SAFI_UNICAST}, Enabled: true,
			}})
		}
	}
	if bfd != nil {
		bfdConfig = &api.BfdPeerConfig{
			Enabled:                  true,
			Port:                     server.BfdServerPort,
			DesiredMinimumTxInterval: bfd.MinimumTransmitMilliseconds * 1000,
			RequiredMinimumReceive:   bfd.MinimumReceiveMilliseconds * 1000,
			DetectionMultiplier:      bfd.Multiplier,
		}
	}
	return ebgpMultihop, families, bfdConfig
}

// probeTCPMD5 sets a password for a prefix on a throwaway socket, the way gobgp does on its listeners.
func probeTCPMD5(prefix netip.Prefix, password string) error {
	bits, keylen := prefix.Bits(), len(password)
	if bits < 0 || bits > 128 || keylen > unix.TCP_MD5SIG_MAXKEYLEN {
		return fmt.Errorf("invalid prefix %s or password", prefix)
	}
	family := unix.AF_INET
	sig := unix.TCPMD5Sig{Flags: unix.TCP_MD5SIG_FLAG_PREFIX, Prefixlen: uint8(bits), Keylen: uint16(keylen)}
	if prefix.Addr().Is4() {
		addr := prefix.Addr().As4()
		copy(sig.Addr.Data[2:], addr[:])
	} else {
		family = unix.AF_INET6
		addr := prefix.Addr().As16()
		copy(sig.Addr.Data[6:], addr[:])
	}
	sig.Addr.Family = uint16(family)
	copy(sig.Key[:], password)

	fd, err := unix.Socket(family, unix.SOCK_STREAM, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	return unix.SetsockoptTCPMD5Sig(fd, unix.IPPROTO_TCP, unix.TCP_MD5SIG_EXT, &sig)
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
func advertiseWhileReady(ctx context.Context, check func(context.Context) error, set func(bool) error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	failures, synced := 3, false
	for {
		select {
		case <-ctx.Done():
			_ = set(false)
			return
		case <-ticker.C:
		}

		if err := check(ctx); err == nil {
			synced = synced && failures < 3
			failures = 0
		} else if ctx.Err() != nil {
			continue
		} else if failures++; failures == 3 {
			logrus.WithField("component", "cplb-bgp").WithError(err).Warn("The local API server isn't ready, withdrawing the virtual IPs")
			synced = false
		}

		if !synced {
			synced = set(failures < 3) == nil
		}
	}
}

// setAdvertised is idempotent, so that failed updates can be retried for all virtual IPs.
func (b *BGP) setAdvertised(advertised bool) (errs error) {
	for _, vip := range b.Config.VirtualIPs {
		prefix, err := netip.ParsePrefix(vip)
		if err != nil {
			b.log.WithError(err).Errorf("Invalid virtual IP %s", vip)
			errs = errors.Join(errs, err)
			continue
		}
		// A preferred lifetime of zero marks the address deprecated, which keeps it from being
		// picked as the node address, or as a source address for IPv6.
		addr := &netlink.Addr{IPNet: &net.IPNet{IP: prefix.Addr().AsSlice(), Mask: net.CIDRMask(prefix.Bits(), prefix.Addr().BitLen())}, ValidLft: math.MaxInt}
		path, err := hostRoute(prefix.Addr(), b.Config.Communities)
		if err != nil {
			b.log.WithError(err).Errorf("Failed to build the route of %s", vip)
			errs = errors.Join(errs, err)
			continue
		}

		if advertised {
			if err = netlink.AddrReplace(b.lo, addr); err == nil {
				_, err = b.server.AddPath(apiutil.AddPathRequest{Paths: []*apiutil.Path{path}})
			}
		} else {
			err = b.server.DeletePath(apiutil.DeletePathRequest{Paths: []*apiutil.Path{path}})
			if delErr := netlink.AddrDel(b.lo, addr); delErr != nil && !errors.Is(delErr, syscall.EADDRNOTAVAIL) {
				err = errors.Join(err, delErr)
			}
		}
		if err != nil {
			b.log.WithError(err).Errorf("Failed to update the advertisement of %s", vip)
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

// hostRoute leaves the next hop unspecified, so that gobgp sets it to the session address.
func hostRoute(addr netip.Addr, communities []string) (*apiutil.Path, error) {
	prefix, err := bgp.NewIPAddrPrefix(netip.PrefixFrom(addr, addr.BitLen()))
	if err != nil {
		return nil, err
	}

	path := &apiutil.Path{Family: bgp.RF_IPv6_UC, Nlri: prefix, Attrs: []bgp.PathAttributeInterface{bgp.NewPathAttributeOrigin(0)}}
	var nextHop bgp.PathAttributeInterface
	if addr.Is4() {
		path.Family = bgp.RF_IPv4_UC
		nextHop, err = bgp.NewPathAttributeNextHop(netip.IPv4Unspecified())
	} else {
		nextHop, err = bgp.NewPathAttributeMpReachNLRI(bgp.RF_IPv6_UC, []bgp.PathNLRI{{NLRI: prefix}}, netip.IPv6Unspecified())
	}
	if err != nil {
		return nil, err
	}
	path.Attrs = append(path.Attrs, nextHop)

	var standard []uint32
	var large []*bgp.LargeCommunity
	for _, community := range communities {
		if wellKnown, ok := bgp.WellKnownCommunityValueMap[community]; ok {
			standard = append(standard, uint32(wellKnown))
		} else if l, largeErr := bgp.ParseLargeCommunity(community); largeErr == nil {
			large = append(large, l)
		} else {
			asn, value, _ := strings.Cut(community, ":")
			high, highErr := strconv.ParseUint(asn, 10, 16)
			low, lowErr := strconv.ParseUint(value, 10, 16)
			if err = errors.Join(highErr, lowErr); err != nil {
				return nil, fmt.Errorf("invalid community %q: %w", community, err)
			}
			standard = append(standard, uint32(high)<<16|uint32(low))
		}
	}
	if len(standard) > 0 {
		path.Attrs = append(path.Attrs, bgp.NewPathAttributeCommunities(standard))
	}
	if len(large) > 0 {
		path.Attrs = append(path.Attrs, bgp.NewPathAttributeLargeCommunities(large))
	}

	return path, nil
}
