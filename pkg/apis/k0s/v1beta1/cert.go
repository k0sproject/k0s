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

package v1beta1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CA defines the certificates related config options
type CA struct {
	// The expiration duration of the CA certificate
	// +kubebuilder:default="87600h"
	ExpiresAfter metav1.Duration `json:"expiresAfter,omitempty"`
	// The expiration duration of the server certificate
	// +kubebuilder:default="8760h"
	CertificatesExpireAfter metav1.Duration `json:"certificatesExpireAfter,omitempty"`
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
