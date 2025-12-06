// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var _ Validateable = (*KonnectivitySpec)(nil)

// KonnectivitySpec defines the requested state for Konnectivity
type KonnectivitySpec struct {
	// admin port to listen on (default 8133)
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=8133
	AdminPort int32 `json:"adminPort,omitempty"`

	// agent port to listen on (default 8132)
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=8132
	AgentPort int32 `json:"agentPort,omitempty"`

	// external address to advertise for the konnectivity agent to connect to
	// +optional
	ExternalAddress string `json:"externalAddress,omitempty"`

	// HostNetwork controls whether the konnectivity agent should use host networking
	// +kubebuilder:default=false
	// +optional
	HostNetwork bool `json:"hostNetwork,omitempty"`
}

// DefaultKonnectivitySpec builds default KonnectivitySpec
func DefaultKonnectivitySpec() *KonnectivitySpec {
	return &KonnectivitySpec{
		AgentPort:   8132,
		AdminPort:   8133,
		HostNetwork: false,
	}
}

// Validate implements [Validateable].
func (k *KonnectivitySpec) Validate() (errs []error) {
	if k == nil {
		return nil
	}

	for _, msg := range validation.IsValidPortNum(int(k.AdminPort)) {
		errs = append(errs, field.Invalid(field.NewPath("adminPort"), k.AdminPort, msg))
	}

	for _, msg := range validation.IsValidPortNum(int(k.AgentPort)) {
		errs = append(errs, field.Invalid(field.NewPath("agentPort"), k.AgentPort, msg))
	}

	return errs
}
