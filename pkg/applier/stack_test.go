// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package applier_test

import (
	"testing"

	"github.com/k0sproject/k0s/pkg/applier"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/yaml"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStack_StripsNamespaceFromClusterScopedResource(t *testing.T) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ns-strip-test",
			Namespace: "should-be-ignored",
		},
	}

	resources, err := applier.ToUnstructured(nil, ns)
	require.NoError(t, err)

	fakes := testutil.NewFakeClientFactory()
	s := applier.Stack{
		Name:      "strip-ns",
		Resources: []*unstructured.Unstructured{resources},
		Clients:   fakes,
	}

	err = s.Apply(t.Context(), true)
	require.NoError(t, err)

	appliedNs, err := fakes.Client.CoreV1().Namespaces().Get(t.Context(), "ns-strip-test", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Empty(t, appliedNs.Namespace)

	lastAppliedAnn, ok := appliedNs.Annotations["k0s.k0sproject.io/last-applied-configuration"]
	require.True(t, ok)
	err = yaml.Unmarshal([]byte(lastAppliedAnn), &appliedNs)
	require.NoError(t, err)
	assert.Empty(t, appliedNs.Namespace)
}
