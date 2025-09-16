// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cplb

import (
	"fmt"
	"net"
	"unsafe"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

func setAddressLabel(ip net.IP, label uint32) error {
	// Use iproute2 as reference for setting the address label.
	// https://git.kernel.org/pub/scm/network/iproute2/iproute2.git/tree/ip/ipaddrlabel.c?h=v4.0.0#n178
	if ip.To4() != nil {
		return fmt.Errorf("cannot set address label for IPv4 address %s", ip)
	}
	req := nl.NewNetlinkRequest(unix.RTM_NEWADDRLABEL,
		unix.NLM_F_REQUEST|unix.NLM_F_ACK|unix.NLM_F_REPLACE|unix.NLM_F_CREATE)

	msg := &IfAddrlabelmsg{
		IfAddrlblmsg: unix.IfAddrlblmsg{
			Family:    uint8(unix.AF_INET6),
			Prefixlen: uint8(128), // The netmask for IPv6. In CPLB we always use /128.
		},
	}

	req.AddData(msg)

	// RFC 3484 and 6724 don't specify label size, but iproute2 uses 32 bit labels:
	// https://git.kernel.org/pub/scm/network/iproute2/iproute2.git/tree/ip/ipaddrlabel.c?h=v4.0.0#n178
	labelBytes := make([]byte, 4)
	nl.NativeEndian().PutUint32(labelBytes, label)

	labelData := nl.NewRtAttr(unix.IFAL_LABEL, labelBytes)
	req.AddData(labelData)

	// Set IFAL_ADDRESS
	addressData := nl.NewRtAttr(unix.IFAL_ADDRESS, []byte(ip.To16()))
	req.AddData(addressData)

	_, err := req.Execute(unix.NETLINK_ROUTE, 0)
	return err
}

// Everything below this line should be defined in
// "github.com/vishvananda/netlink/nl" packages.
// Ideally we'll get it merged into those packages in the future, but for now
// we define it here.

type IfAddrlabelmsg struct {
	unix.IfAddrlblmsg
}

func (msg *IfAddrlabelmsg) Len() int {
	return unix.SizeofIfAddrlblmsg
}

func (msg *IfAddrlabelmsg) Serialize() []byte {
	return (*(*[unix.SizeofIfAddrlblmsg]byte)(unsafe.Pointer(msg)))[:]
}
