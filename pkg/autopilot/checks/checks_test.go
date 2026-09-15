// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package checks_test

import (
	"testing"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/autopilot/checks"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ensures that [checks.CanUpdate] doesn't miss resources whose CRD was created
// after a previous check had already discovered the API resources.
func TestCanUpdate_FindsFreshlyCreatedCRD(t *testing.T) {
	clients := testutil.NewFakeClientFactory()
	log := logrus.New()

	// The cluster doesn't know about the removed API yet, so the update is fine.
	require.NoError(t, checks.CanUpdate(t.Context(), log, clients, "v99.99.99"))

	// Now the CRD gets created, along with an instance of it. The group,
	// version and kind must match the test entry in the removed APIs table.
	k0sExampleGV := schema.GroupVersion{Group: "k0s.k0sproject.example.com", Version: "v1beta1"}
	removedGVK := k0sExampleGV.WithKind("RemovedCRD")
	removedGVR := k0sExampleGV.WithResource("removedcrds")
	clients.DynamicClient.Resources = append(clients.DynamicClient.Resources, &metav1.APIResourceList{
		GroupVersion: k0sExampleGV.String(),
		APIResources: []metav1.APIResource{{
			Name: removedGVR.Resource, Kind: removedGVK.Kind, Namespaced: false,
			Verbs: metav1.Verbs{"get", "list"},
		}},
	})
	removed := &unstructured.Unstructured{}
	removed.SetGroupVersionKind(removedGVK)
	removed.SetName("some-resource")
	_, err := clients.DynamicClient.Resource(removedGVR).Create(t.Context(), removed, metav1.CreateOptions{})
	require.NoError(t, err)

	err = checks.CanUpdate(t.Context(), log, clients, "v99.99.99")
	assert.ErrorContains(t, err,
		removedGVR.Resource+"."+removedGVR.Group+" "+removedGVR.Version+" has been removed in Kubernetes v99.99.99, but there are 1 such resources in the cluster")
}
