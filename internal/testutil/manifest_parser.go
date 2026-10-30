// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"io"
	"iter"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func ParseObjects(scheme *runtime.Scheme, r io.Reader) iter.Seq2[runtime.Object, error] {
	return func(yield func(runtime.Object, error) bool) {
		decoder := yaml.NewYAMLOrJSONDecoder(r, 4096)
		for {
			var resource unstructured.Unstructured
			if err := decoder.Decode(&resource.Object); err != nil {
				if err != io.EOF { //nolint:errorlint
					yield(nil, err)
				}
				return
			}
			if resource.Object == nil {
				continue // empty --- section
			}

			gv := resource.GroupVersionKind().GroupVersion()
			obj, err := scheme.ConvertToVersion(&resource, gv)
			// Fallback to unstructured if the type is unknown.
			if runtime.IsNotRegisteredError(err) {
				obj, err = &resource, nil
			}
			if !yield(obj, err) {
				return
			}
		}
	}
}
