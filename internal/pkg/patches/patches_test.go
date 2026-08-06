// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package patches_test

import (
	"strings"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/patches"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/applier"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func toUnstructured(t *testing.T, manifest []byte) *unstructured.Unstructured {
	t.Helper()
	jsonBytes, err := yaml.YAMLToJSON(manifest)
	require.NoError(t, err)
	obj := &unstructured.Unstructured{}
	require.NoError(t, obj.UnmarshalJSON(jsonBytes))
	return obj
}

const cmManifest = `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: kube-system
data:
  key: value
`

const deployManifest = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: coredns
  namespace: kube-system
spec:
  replicas: 1
`

func TestApply_NoPatches_ReturnsInput(t *testing.T) {
	out, err := patches.Apply([]byte(cmManifest), nil)
	require.NoError(t, err)
	assert.Equal(t, []byte(cmManifest), out)
}

func TestApply_MergePatch(t *testing.T) {
	out, err := patches.Apply([]byte(cmManifest), v1beta1.Patches{{
		Target: v1beta1.PatchTarget{Kind: "ConfigMap", Name: "my-config"},
		Patch:  v1beta1.PatchSpec{Type: v1beta1.MergePatchType, Content: `{"data":{"key":"patched"}}`},
	}})
	require.NoError(t, err)

	got, found, err := unstructured.NestedString(toUnstructured(t, out).Object, "data", "key")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "patched", got)
}

func TestApply_JSONPatch(t *testing.T) {
	out, err := patches.Apply([]byte(deployManifest), v1beta1.Patches{{
		Target: v1beta1.PatchTarget{Kind: "Deployment", Name: "coredns"},
		Patch: v1beta1.PatchSpec{
			Type:    v1beta1.JSONPatchType,
			Content: `[{"op":"replace","path":"/spec/replicas","value":3}]`,
		},
	}})
	require.NoError(t, err)

	replicas, found, err := unstructured.NestedInt64(toUnstructured(t, out).Object, "spec", "replicas")
	require.NoError(t, err)
	require.True(t, found)
	assert.EqualValues(t, 3, replicas)
}

func TestApply_StrategicMerge(t *testing.T) {
	out, err := patches.Apply([]byte(deployManifest), v1beta1.Patches{{
		Target: v1beta1.PatchTarget{Kind: "Deployment", Name: "coredns"},
		Patch: v1beta1.PatchSpec{
			Type:    v1beta1.StrategicMergePatchType,
			Content: "spec:\n  replicas: 5\n",
		},
	}})
	require.NoError(t, err)

	replicas, found, err := unstructured.NestedInt64(toUnstructured(t, out).Object, "spec", "replicas")
	require.NoError(t, err)
	require.True(t, found)
	assert.EqualValues(t, 5, replicas)
}

func TestApply_NamespaceNarrowing(t *testing.T) {
	out, err := patches.Apply([]byte(deployManifest), v1beta1.Patches{{
		Target: v1beta1.PatchTarget{Kind: "Deployment", Name: "coredns", Namespace: "other"},
		Patch:  v1beta1.PatchSpec{Type: v1beta1.MergePatchType, Content: `{"spec":{"replicas":9}}`},
	}})
	require.NoError(t, err)

	// A patch targeting a different namespace must not change the document.
	replicas, found, err := unstructured.NestedInt64(toUnstructured(t, out).Object, "spec", "replicas")
	require.NoError(t, err)
	require.True(t, found)
	assert.EqualValues(t, 1, replicas, "non-matching namespace must leave the doc untouched")
}

func TestApply_MultiDoc_OnlyMatchedDocPatched(t *testing.T) {
	multi := cmManifest + "---\n" + deployManifest
	out, err := patches.Apply([]byte(multi), v1beta1.Patches{{
		Target: v1beta1.PatchTarget{Kind: "ConfigMap", Name: "my-config"},
		Patch:  v1beta1.PatchSpec{Type: v1beta1.MergePatchType, Content: `{"data":{"key":"patched"}}`},
	}})
	require.NoError(t, err)

	s := string(out)
	assert.Contains(t, s, "patched") // ConfigMap changed
	assert.Contains(t, s, "coredns") // Deployment still present
}

func TestApply_StrategicMerge_CRD_Errors(t *testing.T) {
	crd := `apiVersion: example.com/v1
kind: Widget
metadata:
  name: w1
spec:
  size: 1
`
	_, err := patches.Apply([]byte(crd), v1beta1.Patches{{
		Target: v1beta1.PatchTarget{Kind: "Widget", Name: "w1"},
		Patch:  v1beta1.PatchSpec{Type: v1beta1.StrategicMergePatchType, Content: "spec:\n  size: 2\n"},
	}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no registered type")
}

func TestApply_MalformedContent_Errors(t *testing.T) {
	_, err := patches.Apply([]byte(deployManifest), v1beta1.Patches{{
		Target: v1beta1.PatchTarget{Kind: "Deployment", Name: "coredns"},
		Patch:  v1beta1.PatchSpec{Type: v1beta1.JSONPatchType, Content: `not valid json patch`},
	}})
	require.Error(t, err)
}

// multiDocManifest mimics the way kube-proxy builds its manifest: a document is
// written first, then a templated multi-document manifest (which begins with a
// "---" separator) is appended. Each doc must remain independently parseable
// after Apply, even when no patch matches - otherwise the leading separator can
// be lost and the appended docs merge into the preceding one.
const leadingDoc = `apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-proxy
  namespace: kube-system
data:
  config.conf: |-
    apiVersion: kubeproxy.config.k8s.io/v1alpha1
    kind: KubeProxyConfiguration
    clusterCIDR: 10.244.0.0/16
`

const appendedDocs = `
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kube-proxy
  namespace: kube-system
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kube-proxy
  namespace: kube-system
spec:
  template:
    spec:
      containers:
        - name: kube-proxy
          image: kube-proxy:v1
`

// kindNames parses a (possibly multi-document) manifest with the same parser the
// applier uses and returns "<kind>/<name>" for each document found.
func kindNames(t *testing.T, manifest []byte) []string {
	t.Helper()
	resources, err := applier.ReadUnstructuredStream(strings.NewReader(string(manifest)), "test")
	require.NoError(t, err)
	var out []string
	for _, r := range resources {
		out = append(out, r.GetKind()+"/"+r.GetName())
	}
	return out
}

func TestApply_PreservesSeparators_WhenConcatenated_NoMatch(t *testing.T) {
	// No patch targets any doc in the appended manifest.
	patched, err := patches.Apply([]byte(appendedDocs), v1beta1.Patches{{
		Target: v1beta1.PatchTarget{Kind: "Deployment", Name: "coredns", Namespace: "kube-system"},
		Patch:  v1beta1.PatchSpec{Type: v1beta1.MergePatchType, Content: `{"metadata":{"labels":{"x":"y"}}}`},
	}})
	require.NoError(t, err)

	// Simulate kube-proxy: a pre-written doc followed by the processed manifest.
	combined := append([]byte(leadingDoc), patched...)
	assert.Equal(t,
		[]string{"ConfigMap/kube-proxy", "ServiceAccount/kube-proxy", "DaemonSet/kube-proxy"},
		kindNames(t, combined),
		"the ConfigMap must stay a separate document, not merge into the ServiceAccount")
}

func TestApply_PreservesSeparators_WhenConcatenated_WithMatch(t *testing.T) {
	// A patch DOES match one of the appended docs, forcing re-serialization.
	patched, err := patches.Apply([]byte(appendedDocs), v1beta1.Patches{{
		Target: v1beta1.PatchTarget{Kind: "DaemonSet", Name: "kube-proxy", Namespace: "kube-system"},
		Patch:  v1beta1.PatchSpec{Type: v1beta1.MergePatchType, Content: `{"metadata":{"labels":{"patched":"yes"}}}`},
	}})
	require.NoError(t, err)

	combined := append([]byte(leadingDoc), patched...)
	assert.Equal(t,
		[]string{"ConfigMap/kube-proxy", "ServiceAccount/kube-proxy", "DaemonSet/kube-proxy"},
		kindNames(t, combined),
		"the leading separator must survive re-serialization so docs don't merge")
}

func TestApply_OrderedMultiPatch(t *testing.T) {
	out, err := patches.Apply([]byte(deployManifest), v1beta1.Patches{
		{Target: v1beta1.PatchTarget{Kind: "Deployment", Name: "coredns"},
			Patch: v1beta1.PatchSpec{Type: v1beta1.MergePatchType, Content: `{"spec":{"replicas":2}}`}},
		{Target: v1beta1.PatchTarget{Kind: "Deployment", Name: "coredns"},
			Patch: v1beta1.PatchSpec{Type: v1beta1.MergePatchType, Content: `{"spec":{"replicas":7}}`}},
	})
	require.NoError(t, err)
	replicas, found, err := unstructured.NestedInt64(toUnstructured(t, out).Object, "spec", "replicas")
	require.NoError(t, err)
	require.True(t, found)
	assert.EqualValues(t, 7, replicas)
}
