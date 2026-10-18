// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"net"

	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// BGPSpec configures the BGP type of control plane load balancing.
type BGPSpec struct {
	// VirtualIPs are the host prefixes advertised by every ready controller.
	// +kubebuilder:validation:MinItems=1
	VirtualIPs []string `json:"virtualIPs"`

	// RouterID is the BGP router ID, an IPv4 address.
	// Defaults to spec.api.address if that's an IPv4 address.
	RouterID string `json:"routerID,omitempty"`

	// SourceIP is the local address of the BGP sessions.
	SourceIP string `json:"sourceIP,omitempty"`

	// SourceInterface is the interface the BGP sessions are bound to.
	SourceInterface string `json:"sourceInterface,omitempty"`

	// Peers are the BGP peers that the virtual IPs are advertised to.
	// +kubebuilder:validation:MinItems=1
	Peers []BGPPeer `json:"peers"`

	// AS is the autonomous system number of the controllers.
	// +kubebuilder:validation:Minimum=1
	AS uint32 `json:"as"`

	// HoldTimeSeconds is the BGP hold time, at least 3 seconds. Defaults to 90 seconds.
	// +kubebuilder:default=90
	// +kubebuilder:validation:Minimum=3
	HoldTimeSeconds uint32 `json:"holdTimeSeconds,omitempty"`

	// KeepaliveIntervalSeconds is the BGP keepalive interval, it must be
	// shorter than the hold time. Defaults to 30 seconds.
	// +kubebuilder:default=30
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
}

// BFDSpec configures BFD for a BGP peer.
type BFDSpec struct {
	// MinimumReceiveMilliseconds is the minimum interval between received
	// BFD packets. Defaults to 300 milliseconds.
	// +kubebuilder:default=300
	MinimumReceiveMilliseconds uint32 `json:"minimumReceiveMilliseconds,omitempty"`

	// MinimumTransmitMilliseconds is the desired minimum interval between sent
	// BFD packets. Defaults to 300 milliseconds.
	// +kubebuilder:default=300
	MinimumTransmitMilliseconds uint32 `json:"minimumTransmitMilliseconds,omitempty"`

	// Multiplier is the number of missed BFD packets after which the peer is
	// considered down. Defaults to 3.
	// +kubebuilder:default=3
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
		b.KeepaliveIntervalSeconds = 30
	}
	if b.HoldTimeSeconds < 3 {
		errs = append(errs, field.Invalid(path.Child("holdTimeSeconds"), b.HoldTimeSeconds, "must be at least 3"))
	} else if b.KeepaliveIntervalSeconds >= b.HoldTimeSeconds {
		errs = append(errs, field.Invalid(path.Child("keepaliveIntervalSeconds"), b.KeepaliveIntervalSeconds, "must be shorter than holdTimeSeconds"))
	}

	if len(b.Peers) == 0 {
		errs = append(errs, field.Required(path.Child("peers"), ""))
	}
	families, peersValid := make(map[bool]bool), len(b.Peers) > 0
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
		if p.BFD != nil {
			if p.Multihop {
				errs = append(errs, field.Forbidden(peerPath.Child("bfd"), "cannot be combined with multihop"))
			}
			if p.BFD.MinimumReceiveMilliseconds == 0 {
				p.BFD.MinimumReceiveMilliseconds = 300
			}
			if p.BFD.MinimumTransmitMilliseconds == 0 {
				p.BFD.MinimumTransmitMilliseconds = 300
			}
			if p.BFD.Multiplier == 0 {
				p.BFD.Multiplier = 3
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

	return errs
}
