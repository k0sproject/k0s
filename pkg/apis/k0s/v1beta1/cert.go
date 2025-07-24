// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CA defines the certificates related config options
type CA struct {
	// The expiration duration of the CA certificate
	//
	// +kubebuilder:default="87600h"
	// +optional
	ExpiresAfter metav1.Duration `json:"expiresAfter"`
	// The expiration duration of the server certificate
	//
	// +kubebuilder:default="8760h"
	// +optional
	CertificatesExpireAfter metav1.Duration `json:"certificatesExpireAfter"`
}

// DefaultCA returns default settings for CA
func DefaultCA() *CA {
	return &CA{
		ExpiresAfter: metav1.Duration{
			Duration: 87600 * time.Hour,
		},
		CertificatesExpireAfter: metav1.Duration{
			Duration: 8760 * time.Hour,
		},
	}
}
