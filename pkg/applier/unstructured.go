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
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

// ToUnstructured converts the given runtime object to an unstructured one using
// the given scheme. The scheme can be nil, in which case client-go's default
// scheme is used.
func ToUnstructured(s *runtime.Scheme, object runtime.Object) (u *unstructured.Unstructured, err error) {
	u = new(unstructured.Unstructured)
	u.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(object)
	if err != nil {
		return nil, err
	}

	gvk := u.GroupVersionKind()
	if gvk.Group == "" || gvk.Kind == "" {
		if s == nil {
			s = scheme.Scheme
		}

		kinds, _, err := s.ObjectKinds(object)
		if err != nil {
			return nil, err
		}
		apiVersion, kind := kinds[0].ToAPIVersionAndKind()
		u.SetAPIVersion(apiVersion)
		u.SetKind(kind)
	}

	return u, nil
}

// ToUnstructuredSlice converts the given runtime objects to unstructured ones
// using the given scheme. The scheme can be nil, in which case client-go's
// default scheme is used.
func ToUnstructuredSlice[T runtime.Object](s *runtime.Scheme, objects ...T) ([]*unstructured.Unstructured, error) {
	converted := make([]*unstructured.Unstructured, len(objects))
	for i, object := range objects {
		var err error
		converted[i], err = ToUnstructured(s, object)
		if err != nil {
			return nil, fmt.Errorf("at index %d, for some %s: %w", i, object.GetObjectKind().GroupVersionKind(), err)
		}
	}
	return converted, nil
}
