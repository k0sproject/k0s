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

	msg := &IfAddrLabelmsg{
		Family:    uint8(unix.AF_INET6),
		PrefixLen: uint8(128), // The netmask for IPv6. In CPLB we always use /128.
	}

	req.AddData(msg)

	// RFC 3484 and 6724 don't specify label size, but iproute2 uses 32 bit labels:
	// https://git.kernel.org/pub/scm/network/iproute2/iproute2.git/tree/ip/ipaddrlabel.c?h=v4.0.0#n178
	labelBytes := make([]byte, 4)
	nl.NativeEndian().PutUint32(labelBytes, label)

	labelData := nl.NewRtAttr(IFAL_LABEL, labelBytes)
	req.AddData(labelData)

	// Set IFAL_ADDRESS
	addressData := nl.NewRtAttr(IFAL_ADDRESS, []byte(ip.To16()))
	req.AddData(addressData)

	_, err := req.Execute(unix.NETLINK_ROUTE, 0)
	return err
}

// Everything below this line should be defined in "golang.org/x/sys/unix"
// and "github.com/vishvananda/netlink/nl" packages.
// Ideally we'll get it merged into those packages in the future, but for now
// define it here.

type IfAddrLabelmsg struct {
	Family    uint8  // Address family
	_         uint8  // Reserved
	PrefixLen uint8  // Prefix length
	Flags     uint8  // Flags
	Index     uint32 // Link index
	Seq       uint32 // Sequence number
}

const (
	IFAL_ADDRESS = 1
	IFAL_LABEL   = 2
)

const sizeOfIfAddrLabelMsg = int(unsafe.Sizeof(IfAddrLabelmsg{}))

func (msg *IfAddrLabelmsg) Len() int {
	return sizeOfIfAddrLabelMsg
}

func (msg *IfAddrLabelmsg) Serialize() []byte {
	return (*(*[sizeOfIfAddrLabelMsg]byte)(unsafe.Pointer(msg)))[:]
}

// https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git/tree/include/uapi/linux/if_addrlabel.h?h=v4.0#n15
// struct ifaddrlblmsg {
//	__u8		ifal_family;		/* Address family */
//	__u8		__ifal_reserved;	/* Reserved */
//	__u8		ifal_prefixlen;		/* Prefix length */
//	__u8		ifal_flags;		/* Flags */
//	__u32		ifal_index;		/* Link index */
//	__u32		ifal_seq;		/* sequence number */
// };

// enum {
//	IFAL_ADDRESS = 1,
//	IFAL_LABEL = 2,
//	__IFAL_MAX
// };
