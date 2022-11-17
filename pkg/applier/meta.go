/*
Copyright 2022 k0s authors

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

package applier

import (
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
	for n, v := range m {
		combined[n] = v
	}
	for n, v := range other {
		combined[n] = v
	}

	return combined
}
