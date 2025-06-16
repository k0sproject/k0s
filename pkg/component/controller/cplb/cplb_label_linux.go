/*
Copyright 2025 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cplb

import (
	"fmt"
	"net"
	"unsafe"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// maybeSetAddrLabels sets the labels for the vips to vrrp.AddressLabel
// so that the real IP is preferred.
func (k *Keepalived) maybeSetAddrLabels() error {
	for _, vrrp := range k.Config.VRRPInstances {
		for _, vip := range vrrp.VirtualIPs {
			// Only set labels for IPv6 addresses
			ipAddr, _, err := net.ParseCIDR(vip)
			if err != nil {
				return fmt.Errorf("failed to parse CIDR %s: %w", vip, err)
			}

			// Only set labels for IPv6 addresses
			if ipAddr.To4() != nil {
				continue
			}

			// Set address label for IPv6 VIP
			if err := setAddressLabel(ipAddr, 128, vrrp.AddressLabel); err != nil {
				return fmt.Errorf("failed to set address label for %s: %w", ipAddr, err)
			}
		}
	}
	return nil
}

func setAddressLabel(ip net.IP, bits int, label uint32) error {
	// Use iproute2 as reference for setting the address label.
	// https://kernel.googlesource.com/pub/scm/network/iproute2/iproute2/+/refs/tags/v6.15.0/ip/ipaddrlabel.c#176
	if ip.To4() != nil {
		return fmt.Errorf("cannot set address label for IPv4 address %s", ip)
	}
	req := nl.NewNetlinkRequest(unix.RTM_NEWADDRLABEL,
		unix.NLM_F_REQUEST|unix.NLM_F_ACK|unix.NLM_F_REPLACE|unix.NLM_F_CREATE)

	msg := &IfAddrLabelmsg{
		Family:    uint8(unix.AF_INET6),
		PrefixLen: uint8(bits),
	}

	req.AddData(msg)

	// RFC 3484 and 6724 don't specify label size, but iproute2 uses 32 bit labels:
	// https://kernel.googlesource.com/pub/scm/network/iproute2/iproute2/+/refs/tags/v6.15.0/ip/ipaddrlabel.c#176
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
	Family    uint8
	_         uint8
	PrefixLen uint8
	Flags     uint8
	Index     uint32
	Seq       uint32
}

const (
	IFAL_ADDRESS = 1
	IFAL_LABEL   = 2
)

const SizeOfIfAddrLabelMsg = 12 // Size of IfAddrLabelMsg in bytes

func (msg *IfAddrLabelmsg) Len() int {
	return SizeOfIfAddrLabelMsg
}

func (msg *IfAddrLabelmsg) Serialize() []byte {
	return (*(*[SizeOfIfAddrLabelMsg]byte)(unsafe.Pointer(msg)))[:]
}

// https://github.com/torvalds/linux/blob/v6.15/include/uapi/linux/if_addrlabel.h
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
