// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"net"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/asaskevich/govalidator"
)

// NodeAddresses restricts the names and addresses that nodes may obtain
// kubelet serving certificates for.
//
// A node may write arbitrary addresses into its own status, and k0s's
// csr-approver signs kubelet serving certificates for the addresses that a node
// reports for itself. Setting this bounds what a node can vouch for.
type NodeAddresses struct {
	// CIDRs that contain the IP addresses of the cluster's nodes. If set,
	// csr-approver refuses kubelet serving certificates for IP addresses
	// outside of them. If empty, IP addresses are unrestricted.
	// +optional
	CIDRs []string `json:"cidrs,omitempty"`

	// DNSNames lists patterns for the DNS names of the cluster's nodes. If set,
	// csr-approver refuses kubelet serving certificates for DNS names that
	// match none of them. If empty, DNS names are unrestricted. Patterns are
	// matched case-insensitively. An asterisk matches any sequence of
	// characters within a single DNS label, i.e. it doesn't match dots.
	// +optional
	DNSNames []string `json:"dnsNames,omitempty"`
}

func (n *NodeAddresses) Validate(path *field.Path) (errs field.ErrorList) {
	if n == nil {
		return nil
	}

	for i, cidr := range n.CIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			errs = append(errs, field.Invalid(path.Child("cidrs").Index(i), cidr, "invalid CIDR address"))
		}
	}

	for i, pattern := range n.DNSNames {
		// Substitute wildcards with a letter, so that patterns like
		// "*.example.com" or "node-*" are validated like DNS names.
		if !govalidator.IsDNSName(strings.ReplaceAll(pattern, "*", "x")) {
			errs = append(errs, field.Invalid(path.Child("dnsNames").Index(i), pattern, "invalid DNS name pattern"))
		}
	}

	return errs
}
