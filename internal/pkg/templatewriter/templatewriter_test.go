// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package templatewriter_test

import (
	"bytes"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const tmpl = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: coredns
  namespace: kube-system
spec:
  replicas: {{ .Replicas }}
`

func TestTemplateWriter_AppliesPatches(t *testing.T) {
	tw := templatewriter.TemplateWriter{
		Name:     "coredns",
		Template: tmpl,
		Data:     struct{ Replicas int }{Replicas: 1},
		Patches: v1beta1.Patches{{
			Target: v1beta1.PatchTarget{Kind: "Deployment", Name: "coredns"},
			Patch:  v1beta1.PatchSpec{Type: v1beta1.MergePatchType, Content: `{"spec":{"replicas":4}}`},
		}},
	}

	var buf bytes.Buffer
	require.NoError(t, tw.WriteToBuffer(&buf))

	jsonBytes, err := yaml.YAMLToJSON(buf.Bytes())
	require.NoError(t, err)
	obj := &unstructured.Unstructured{}
	require.NoError(t, obj.UnmarshalJSON(jsonBytes))

	replicas, found, err := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	require.NoError(t, err)
	require.True(t, found)
	assert.EqualValues(t, 4, replicas)
}

func TestTemplateWriter_NoPatches_Unchanged(t *testing.T) {
	tw := templatewriter.TemplateWriter{
		Name:     "coredns",
		Template: tmpl,
		Data:     struct{ Replicas int }{Replicas: 1},
	}
	var buf bytes.Buffer
	require.NoError(t, tw.WriteToBuffer(&buf))
	assert.Contains(t, buf.String(), "replicas: 1")
}
