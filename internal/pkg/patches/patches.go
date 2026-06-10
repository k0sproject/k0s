// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

// Package patches applies user-defined customizations to rendered k0s
// manifests before they are written to disk.
package patches

import (
	"bytes"
	"fmt"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/yaml"
)

// docSeparator separates documents in the emitted multi-document YAML stream.
var docSeparator = []byte("\n---\n")

// Apply runs the given patches against a (possibly multi-document) rendered
// YAML manifest, returning the patched manifest. With no patches, the input is
// returned unchanged.
func Apply(rendered []byte, patches v1beta1.Patches) ([]byte, error) {
	if len(patches) == 0 {
		return rendered, nil
	}

	docs := splitYAML(rendered)
	outDocs := make([][]byte, 0, len(docs))
	for _, doc := range docs {
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}
		patched, err := applyToDoc(doc, patches)
		if err != nil {
			return nil, err
		}
		outDocs = append(outDocs, patched)
	}

	return append([]byte("---\n"), bytes.Join(outDocs, docSeparator)...), nil
}

// applyToDoc applies every matching patch (in order) to a single YAML document.
// Documents that match no patch are returned verbatim.
func applyToDoc(doc []byte, patches v1beta1.Patches) ([]byte, error) {
	obj := &unstructured.Unstructured{}
	jsonDoc, err := yaml.YAMLToJSON(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest document: %w", err)
	}
	if err := obj.UnmarshalJSON(jsonDoc); err != nil {
		return nil, fmt.Errorf("failed to decode manifest document: %w", err)
	}

	kind := obj.GetKind()
	name := obj.GetName()
	namespace := obj.GetNamespace()

	current := jsonDoc
	matched := false
	for i, p := range patches {
		if p.Target.Kind != kind || p.Target.Name != name {
			continue
		}
		if p.Target.Namespace != "" && p.Target.Namespace != namespace {
			continue
		}
		matched = true

		patchJSON, err := yaml.YAMLToJSON([]byte(p.Patch.Content))
		if err != nil {
			return nil, fmt.Errorf("patches[%d]: invalid content: %w", i, err)
		}

		switch p.Patch.Type {
		case v1beta1.JSONPatchType:
			current, err = applyJSONPatch(current, patchJSON)
		case v1beta1.MergePatchType:
			current, err = jsonpatch.MergePatch(current, patchJSON)
		case v1beta1.StrategicMergePatchType:
			current, err = applyStrategicMerge(obj.GroupVersionKind(), current, patchJSON)
		default:
			err = fmt.Errorf("unknown patch type %q", p.Patch.Type)
		}
		if err != nil {
			return nil, fmt.Errorf("error patching resource (%s/%s): %w", kind, name, err)
		}
	}

	if !matched {
		return doc, nil
	}
	return yaml.JSONToYAML(current)
}

func applyJSONPatch(original, patchJSON []byte) ([]byte, error) {
	patch, err := jsonpatch.DecodePatch(patchJSON)
	if err != nil {
		return nil, fmt.Errorf("invalid json patch: %w", err)
	}
	return patch.Apply(original)
}

func applyStrategicMerge(gvk schema.GroupVersionKind, original, patchJSON []byte) ([]byte, error) {
	typed, err := kubescheme.Scheme.New(gvk)
	if err != nil {
		return nil, fmt.Errorf("strategic merge unsupported for %s: no registered type", gvk)
	}
	return strategicpatch.StrategicMergePatch(original, patchJSON, typed)
}

// splitYAML splits a multi-document YAML byte stream into individual documents.
func splitYAML(in []byte) [][]byte {
	return bytes.Split(in, []byte("\n---"))
}
