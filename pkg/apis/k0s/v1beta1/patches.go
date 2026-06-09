// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Patches is a list of user-defined customizations applied to k0s-generated
// resources before they are written and applied.
type Patches []Patch

// Patch is a single customization targeting one generated resource.
type Patch struct {
	// Target selects which generated resource to patch.
	// +kubebuilder:validation:Required
	Target PatchTarget `json:"target"`
	// Patch defines the patch type and content.
	// +kubebuilder:validation:Required
	Patch PatchSpec `json:"patch"`
}

// PatchTarget selects a generated resource by Kind and Name, optionally
// narrowed to a Namespace.
type PatchTarget struct {
	// Kind is the Kubernetes Kind of the target resource
	// (e.g. "Deployment", "Service", "ConfigMap").
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
	// Name is the metadata.name of the target resource.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Namespace optionally narrows the match to a namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// PatchSpec is the patch type and body.
type PatchSpec struct {
	// Type is the patch type to apply.
	// +kubebuilder:validation:Enum=json;strategic;merge
	// +kubebuilder:validation:Required
	Type PatchType `json:"type"`
	// Content is the patch body (JSON or YAML; YAML is converted to JSON).
	// +kubebuilder:validation:Required
	Content string `json:"content"`
}

// PatchType is the supported patch encoding.
type PatchType string

const (
	// JSONPatchType is RFC 6902 JSON Patch (array of operations).
	JSONPatchType PatchType = "json"
	// StrategicMergePatchType is Kubernetes strategic merge patch.
	StrategicMergePatchType PatchType = "strategic"
	// MergePatchType is RFC 7386 JSON Merge Patch.
	MergePatchType PatchType = "merge"
)

// Validate checks every patch for a known type and a non-empty target.
func (p Patches) Validate() []error {
	return p.validate(field.NewPath("patches"))
}

// validate checks every patch for a known type and a non-empty target,
// reporting errors relative to the given field path.
func (p Patches) validate(path *field.Path) (errs []error) {
	for i, patch := range p {
		item := path.Index(i)
		switch patch.Patch.Type {
		case JSONPatchType, StrategicMergePatchType, MergePatchType:
			// valid
		default:
			errs = append(errs, fmt.Errorf("%s: invalid type %q: must be one of json, strategic, merge", item.Child("patch", "type"), patch.Patch.Type))
		}
		if patch.Target.Kind == "" {
			errs = append(errs, fmt.Errorf("%s is required", item.Child("target", "kind")))
		}
		if patch.Target.Name == "" {
			errs = append(errs, fmt.Errorf("%s is required", item.Child("target", "name")))
		}
	}
	return errs
}
