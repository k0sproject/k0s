// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"net/netip"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/asaskevich/govalidator"
)

// Defines the set of addresses that a node is allowed to claim.
type NodeAddresses struct {
	// CIDRs that contain the IP addresses of the cluster's nodes. If empty, IP
	// addresses are unrestricted.
	// +optional
	CIDRs []NodeCIDR `json:"cidrs,omitempty"`

	// DNSNames lists patterns for the DNS names of the cluster's nodes. If
	// empty, DNS names are unrestricted.
	// +optional
	DNSNames []NodeDNSName `json:"dnsNames,omitempty"`
}

// A CIDR that contains the IP addresses of nodes.
// +kubebuilder:validation:MaxLength=49
// +kubebuilder:validation:Pattern=`^[0-9a-fA-F:.]+/[0-9]{1,3}$`
type NodeCIDR string

// A DNS name pattern for the names of nodes. Asterisks are allowed in place of
// characters.
// +kubebuilder:validation:MaxLength=253
// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9*]([a-zA-Z0-9*-]*[a-zA-Z0-9*])?(\.[a-zA-Z0-9*]([a-zA-Z0-9*-]*[a-zA-Z0-9*])?)*\.?$`
type NodeDNSName string

func (n NodeAddresses) Validate(path *field.Path) (errs field.ErrorList) {
	for i, cidr := range n.CIDRs {
		// IPv4-mapped IPv6 prefixes would never match an unmapped address.
		if prefix, err := netip.ParsePrefix(string(cidr)); err != nil || prefix.Addr().Is4In6() {
			errs = append(errs, field.Invalid(path.Child("cidrs").Index(i), string(cidr), "invalid CIDR address"))
		}
	}

	for i, pattern := range n.DNSNames {
		// Substitute wildcards with a letter, so that patterns like
		// "*.example.com" or "node-*" are validated like DNS names.
		if !govalidator.IsDNSName(strings.ReplaceAll(string(pattern), "*", "x")) {
			errs = append(errs, field.Invalid(path.Child("dnsNames").Index(i), string(pattern), "invalid DNS name pattern"))
		}
	}

	return errs
}
