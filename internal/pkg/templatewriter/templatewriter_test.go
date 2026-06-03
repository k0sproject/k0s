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

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(buf.Bytes(), &parsed))
	assert.EqualValues(t, 4, parsed["spec"].(map[string]any)["replicas"])
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
