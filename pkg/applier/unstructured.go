// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package applier

import (
	"errors"
	"fmt"
	"io"
	"io/fs"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
)

// Reads and parses resources read from src, returning them as a slice of
// unstructured objects.
func ReadUnstructuredStream(src io.Reader, name string) ([]*unstructured.Unstructured, error) {
	infos, err := resource.NewLocalBuilder().
		Unstructured().
		Stream(src, name).
		Flatten().
		Do().
		Infos()
	if err != nil {
		return nil, err
	}

	resources := make([]*unstructured.Unstructured, len(infos))
	for i := range infos {
		resources[i] = infos[i].Object.(*unstructured.Unstructured)
	}
	return resources, nil
}

func ReadUnstructuredDir(fsys fs.FS, path string) (resources []*unstructured.Unstructured, _ error) {
	err := fs.WalkDir(fsys, path, func(path string, d fs.DirEntry, walkErr error) (err error) {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			return nil
		}

		r, err := fsys.Open(path)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, r.Close()) }()

		fileResources, err := ReadUnstructuredStream(r, path)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}

		resources = append(resources, fileResources...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return resources, nil
}

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
