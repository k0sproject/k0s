// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

// CoreDNS defines the configuration options for the CoreDNS component.
type CoreDNS struct {
	// Patches holds customizations applied to the CoreDNS resources generated
	// by k0s before they are written and applied.
	// +optional
	Patches Patches `json:"patches,omitempty"`
}
