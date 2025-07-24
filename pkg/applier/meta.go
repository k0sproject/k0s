// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package applier

import (
	"maps"
	"strings"

	"github.com/k0sproject/k0s/pkg/build"
)

const (
	// MetaPrefix is the prefix to all label and annotation names unique to k0s.
	MetaPrefix = "k0s.k0sproject.io"

	// NameLabel stack label
	NameLabel = MetaPrefix + "/stack"

	// ChecksumAnnotation defines the annotation key to used for stack checksums
	ChecksumAnnotation = MetaPrefix + "/stack-checksum"

	// LastConfigAnnotation defines the annotation to be used for last applied configs
	LastConfigAnnotation = MetaPrefix + "/last-applied-configuration"
)

// Meta is a convenience wrapper for metav1.ObjectMeta.Labels and
// metav1.ObjectMeta.Annotations.
type Meta map[string]string

var metaVersionValue = strings.ReplaceAll(build.Version, "+", "-")

// CommonLabels returns a set of common labels to be set on all k0s-owned resources.
//
// See https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/#labels
func CommonLabels(componentName string) Meta {
	return map[string]string{
		"app.kubernetes.io/name":       "k0s",
		"app.kubernetes.io/component":  componentName,
		"app.kubernetes.io/version":    metaVersionValue,
		"app.kubernetes.io/managed-by": "k0s",
	}
}

// WithAll returns a copy of this Meta with the given value added to it,
// possibly overwriting a previous value.
func (m Meta) With(name, value string) Meta {
	return m.WithAll(map[string]string{name: value})
}

// WithAll returns a copy of this Meta with other added to it, overwriting any
// duplicates.
func (m Meta) WithAll(other map[string]string) Meta {
	combined := make(Meta, len(m)+len(other))
	maps.Copy(combined, m)
	maps.Copy(combined, other)

	return combined
}
