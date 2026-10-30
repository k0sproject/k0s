// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"math"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// BGPSpec configures the BGP type of control plane load balancing.
type BGPSpec struct {
	// Listen accepts BGP sessions from peers within the configured prefixes.
	Listen *BGPListen `json:"listen,omitempty"`

	// VirtualIPs are the host prefixes advertised by every ready controller.
	// +kubebuilder:validation:MinItems=1
	VirtualIPs []string `json:"virtualIPs"`

	// Communities are added to the routes of the virtual IPs, as standard (a:b) or
	// large (a:b:c) communities, or well-known ones such as no-export.
	Communities []string `json:"communities,omitempty"`

	// RouterID is the BGP router ID, an IPv4 address.
	// Defaults to spec.api.address if that's an IPv4 address.
	RouterID string `json:"routerID,omitempty"`

	// SourceIP is the local address of the BGP sessions.
	SourceIP string `json:"sourceIP,omitempty"`

	// SourceInterface is the interface the BGP sessions are bound to.
	SourceInterface string `json:"sourceInterface,omitempty"`

	// Peers are the BGP peers that the virtual IPs are advertised to.
	Peers []BGPPeer `json:"peers,omitempty"`

	// AS is the autonomous system number of the controllers.
	// +kubebuilder:validation:Minimum=1
	AS uint32 `json:"as"`

	// HoldTimeSeconds is the BGP hold time, between 3 and 65535 seconds. Defaults to 90 seconds.
	// +kubebuilder:default=90
	// +kubebuilder:validation:Minimum=3
	// +kubebuilder:validation:Maximum=65535
	HoldTimeSeconds uint32 `json:"holdTimeSeconds,omitempty"`

	// KeepaliveIntervalSeconds is the BGP keepalive interval, it must be
	// shorter than the hold time. Defaults to a third of the hold time.
	KeepaliveIntervalSeconds uint32 `json:"keepaliveIntervalSeconds,omitempty"`
}

// BGPPeer is a BGP peer of the controllers.
type BGPPeer struct {
	// BFD enables BFD for the peer, which can't be combined with multihop.
	BFD *BFDSpec `json:"bfd,omitempty"`

	// Address is the IP address of the peer.
	Address string `json:"address"`

	// Password is the TCP MD5 password of the session.
	Password string `json:"password,omitempty"`

	// AS is the autonomous system number of the peer.
	// +kubebuilder:validation:Minimum=1
	AS uint32 `json:"as"`

	// Port is the BGP port of the peer. Defaults to 179.
	// +kubebuilder:default=179
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port uint32 `json:"port,omitempty"`

	// Multihop allows the peer to be more than one hop away.
	Multihop bool `json:"multihop,omitempty"`

	// ExtendedNextHop advertises the IPv4 virtual IPs to this IPv6 peer, with an IPv6
	// next hop as defined in RFC 8950.
	ExtendedNextHop bool `json:"extendedNextHop,omitempty"`
}

// BGPListen configures the controllers to accept BGP sessions.
type BGPListen struct {
	// Addresses are the local addresses to listen on. Defaults to all addresses.
	Addresses []string `json:"addresses,omitempty"`

	// Neighbors are the prefixes that BGP sessions are accepted from.
	// +kubebuilder:validation:MinItems=1
	Neighbors []BGPNeighbor `json:"neighbors"`

	// Port is the TCP port to listen on.
	// +kubebuilder:validation:Minimum=1
	Port uint16 `json:"port"`
}

// BGPNeighbor accepts BGP sessions from the addresses within a prefix.
type BGPNeighbor struct {
	// BFD enables BFD for the neighbors, which can't be combined with multihop.
	BFD *BFDSpec `json:"bfd,omitempty"`

	// Prefix contains the addresses that sessions are accepted from.
	Prefix string `json:"prefix"`

	// Password is the TCP MD5 password of the sessions.
	Password string `json:"password"`

	// AS is the autonomous system number of the neighbors.
	// +kubebuilder:validation:Minimum=1
	AS uint32 `json:"as"`

	// Multihop allows the neighbors to be more than one hop away.
	Multihop bool `json:"multihop,omitempty"`

	// ExtendedNextHop advertises the IPv4 virtual IPs to neighbors of an IPv6
	// prefix, with an IPv6 next hop as defined in RFC 8950.
	ExtendedNextHop bool `json:"extendedNextHop,omitempty"`
}

// BFDSpec configures BFD for a BGP peer.
type BFDSpec struct {
	// MinimumReceiveMilliseconds is the minimum interval between received
	// BFD packets. Defaults to 300 milliseconds.
	// +kubebuilder:default=300
	// +kubebuilder:validation:Maximum=4294967
	MinimumReceiveMilliseconds uint32 `json:"minimumReceiveMilliseconds,omitempty"`

	// MinimumTransmitMilliseconds is the desired minimum interval between sent
	// BFD packets. Defaults to 300 milliseconds.
	// +kubebuilder:default=300
	// +kubebuilder:validation:Maximum=4294967
	MinimumTransmitMilliseconds uint32 `json:"minimumTransmitMilliseconds,omitempty"`

	// Multiplier is the number of missed BFD packets after which the peer is
	// considered down. Defaults to 3.
	// +kubebuilder:default=3
	// +kubebuilder:validation:Maximum=255
	Multiplier uint32 `json:"multiplier,omitempty"`
}

// Validate validates the BGPSpec and sets its defaults.
func (b *BGPSpec) Validate(path *field.Path) (errs field.ErrorList) {
	if b == nil {
		return field.ErrorList{field.Required(path, "when type is BGP")}
	}

	if b.AS == 0 {
		errs = append(errs, field.Required(path.Child("as"), ""))
	}
	if b.RouterID != "" {
		if ipErrs := validation.IsValidIP(path.Child("routerID"), b.RouterID); len(ipErrs) > 0 {
			errs = append(errs, ipErrs...)
		} else if net.ParseIP(b.RouterID).To4() == nil {
			errs = append(errs, field.Invalid(path.Child("routerID"), b.RouterID, "must be an IPv4 address"))
		}
	}
	if b.SourceIP != "" {
		errs = append(errs, validation.IsValidIP(path.Child("sourceIP"), b.SourceIP)...)
		if b.SourceInterface != "" {
			errs = append(errs, field.Forbidden(path.Child("sourceInterface"), "cannot be combined with sourceIP"))
		}
	}

	if b.HoldTimeSeconds == 0 {
		b.HoldTimeSeconds = 90
	}
	if b.KeepaliveIntervalSeconds == 0 {
		b.KeepaliveIntervalSeconds = b.HoldTimeSeconds / 3
	}
	if b.HoldTimeSeconds < 3 || b.HoldTimeSeconds > math.MaxUint16 {
		errs = append(errs, field.Invalid(path.Child("holdTimeSeconds"), b.HoldTimeSeconds, "must be between 3 and 65535"))
	} else if b.KeepaliveIntervalSeconds >= b.HoldTimeSeconds {
		errs = append(errs, field.Invalid(path.Child("keepaliveIntervalSeconds"), b.KeepaliveIntervalSeconds, "must be shorter than holdTimeSeconds"))
	}

	neighbors := 0
	if b.Listen != nil {
		neighbors = len(b.Listen.Neighbors)
	}
	if len(b.Peers) == 0 && neighbors == 0 {
		errs = append(errs, field.Required(path.Child("peers"), "unless listen neighbors are set"))
	}
	families, peersValid := make(map[bool]bool), len(b.Peers)+neighbors > 0
	for i := range b.Peers {
		p, peerPath := &b.Peers[i], path.Child("peers").Index(i)
		if ipErrs := validation.IsValidIP(peerPath.Child("address"), p.Address); len(ipErrs) > 0 {
			errs = append(errs, ipErrs...)
			peersValid = false
		} else if ip := net.ParseIP(p.Address); ip.IsLinkLocalUnicast() {
			errs = append(errs, field.Invalid(peerPath.Child("address"), p.Address, "must not be a link-local address"))
			peersValid = false
		} else {
			families[ip.To4() == nil] = true
			families[false] = families[false] || p.ExtendedNextHop
			errs = append(errs, validateSession(peerPath, p.BFD, p.Multihop, p.ExtendedNextHop, ip.To4() == nil)...)
		}
		if p.AS == 0 {
			errs = append(errs, field.Required(peerPath.Child("as"), ""))
		}
		if p.Port == 0 {
			p.Port = 179
		}
		for _, msg := range validation.IsValidPortNum(int(p.Port)) {
			errs = append(errs, field.Invalid(peerPath.Child("port"), p.Port, msg))
		}
		if len(p.Password) > 80 {
			errs = append(errs, field.TooLong(peerPath.Child("password"), "", 80))
		}
	}

	if b.Listen != nil {
		listenPath := path.Child("listen")
		for _, msg := range validation.IsValidPortNum(int(b.Listen.Port)) {
			errs = append(errs, field.Invalid(listenPath.Child("port"), b.Listen.Port, msg))
		}
		for i, address := range b.Listen.Addresses {
			errs = append(errs, validation.IsValidIP(listenPath.Child("addresses").Index(i), address)...)
		}
		if neighbors == 0 {
			errs = append(errs, field.Required(listenPath.Child("neighbors"), ""))
		}
		seen := make(map[netip.Prefix]bool)
		for i := range b.Listen.Neighbors {
			n, neighborPath := &b.Listen.Neighbors[i], listenPath.Child("neighbors").Index(i)
			switch prefix, err := netip.ParsePrefix(n.Prefix); {
			case err != nil:
				errs = append(errs, field.Invalid(neighborPath.Child("prefix"), n.Prefix, "must be a CIDR"))
				peersValid = false
			case prefix != prefix.Masked():
				errs = append(errs, field.Invalid(neighborPath.Child("prefix"), n.Prefix, "must not have host bits set"))
				peersValid = false
			case prefix.Addr().IsLinkLocalUnicast():
				errs = append(errs, field.Invalid(neighborPath.Child("prefix"), n.Prefix, "must not be a link-local prefix"))
				peersValid = false
			case seen[prefix]:
				errs = append(errs, field.Duplicate(neighborPath.Child("prefix"), n.Prefix))
			default:
				seen[prefix] = true
				families[prefix.Addr().Is6()] = true
				families[false] = families[false] || n.ExtendedNextHop
				errs = append(errs, validateSession(neighborPath, n.BFD, n.Multihop, n.ExtendedNextHop, prefix.Addr().Is6())...)
			}
			if n.AS == 0 {
				errs = append(errs, field.Required(neighborPath.Child("as"), ""))
			}
			if n.Password == "" {
				errs = append(errs, field.Required(neighborPath.Child("password"), "sessions accepted from a prefix must be authenticated"))
			} else if len(n.Password) > 80 {
				errs = append(errs, field.TooLong(neighborPath.Child("password"), "", 80))
			}
		}
	}

	if len(b.VirtualIPs) == 0 {
		errs = append(errs, field.Required(path.Child("virtualIPs"), ""))
	}
	for i, address := range b.VirtualIPs {
		if ipErrs := validation.IsValidInterfaceAddress(path.Child("virtualIPs").Index(i), address); len(ipErrs) > 0 {
			errs = append(errs, ipErrs...)
			continue
		}
		ip, ipNet, _ := net.ParseCIDR(address)
		if ones, bits := ipNet.Mask.Size(); ones != bits {
			errs = append(errs, field.Invalid(path.Child("virtualIPs").Index(i), address, "must be a host prefix"))
		}
		if peersValid && !families[ip.To4() == nil] {
			errs = append(errs, field.Invalid(path.Child("virtualIPs").Index(i), address, "no BGP peer of the same IP family"))
		}
	}

	for i, community := range b.Communities {
		valid := slices.Contains([]string{"no-export", "no-advertise", "no-export-subconfed", "no-peer", "blackhole"}, community)
		if parts := strings.Split(community, ":"); len(parts) == 2 || len(parts) == 3 {
			valid = true
			for _, part := range parts {
				if _, err := strconv.ParseUint(part, 10, 16*(len(parts)-1)); err != nil {
					valid = false
				}
			}
		}
		if !valid {
			errs = append(errs, field.Invalid(path.Child("communities").Index(i), community, "must be a:b, a:b:c or a well-known community"))
		}
	}

	return errs
}

// validateSession validates the options shared by peers and listen neighbors, and sets their defaults.
func validateSession(path *field.Path, bfd *BFDSpec, multihop, extendedNextHop, ipv6 bool) (errs field.ErrorList) {
	if extendedNextHop && !ipv6 {
		errs = append(errs, field.Invalid(path.Child("extendedNextHop"), extendedNextHop, "requires an IPv6 peer"))
	}
	if bfd != nil {
		if multihop {
			errs = append(errs, field.Forbidden(path.Child("bfd"), "cannot be combined with multihop"))
		}
		if bfd.MinimumReceiveMilliseconds == 0 {
			bfd.MinimumReceiveMilliseconds = 300
		}
		if bfd.MinimumTransmitMilliseconds == 0 {
			bfd.MinimumTransmitMilliseconds = 300
		}
		if bfd.Multiplier == 0 {
			bfd.Multiplier = 3
		}
		if bfd.MinimumReceiveMilliseconds > math.MaxUint32/1000 {
			errs = append(errs, field.Invalid(path.Child("bfd", "minimumReceiveMilliseconds"), bfd.MinimumReceiveMilliseconds, "must be at most 4294967"))
		}
		if bfd.MinimumTransmitMilliseconds > math.MaxUint32/1000 {
			errs = append(errs, field.Invalid(path.Child("bfd", "minimumTransmitMilliseconds"), bfd.MinimumTransmitMilliseconds, "must be at most 4294967"))
		}
		if bfd.Multiplier > math.MaxUint8 {
			errs = append(errs, field.Invalid(path.Child("bfd", "multiplier"), bfd.Multiplier, "must be at most 255"))
		}
	}
	return errs
}
