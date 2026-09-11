// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package checks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/k0sproject/k0s/internal/testutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	removedGroup    = "k0s.k0sproject.example.com"
	removedVersion  = "v1beta1"
	removedKind     = "RemovedCRD"
	removedResource = "removedcrds"
)

// restConfigClientFactory adds a working REST config to the fake client
// factory, so that [CanUpdate] can build its metadata client.
type restConfigClientFactory struct {
	*testutil.FakeClientFactory
	restConfig *rest.Config
}

func (f *restConfigClientFactory) GetRESTConfig() (*rest.Config, error) {
	return f.restConfig, nil
}

// TestCanUpdate_FreshlyCreatedCRD ensures that the removed API check doesn't
// miss resources whose CRD was created after the discovery data had been
// cached. Missing them lets an update proceed that should have been stopped.
//
// See https://github.com/k0sproject/k0s/issues/8241
func TestCanUpdate_FreshlyCreatedCRD(t *testing.T) {
	// Serve a single instance of the removed resource, and nothing else.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var items []metav1.PartialObjectMetadata
		if r.URL.Path == "/apis/"+removedGroup+"/"+removedVersion+"/"+removedResource {
			items = []metav1.PartialObjectMetadata{{
				ObjectMeta: metav1.ObjectMeta{Name: "removed-resource"},
			}}
		}
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(&metav1.PartialObjectMetadataList{
			TypeMeta: metav1.TypeMeta{APIVersion: "meta.k8s.io/v1", Kind: "PartialObjectMetadataList"},
			Items:    items,
		}))
	}))
	t.Cleanup(server.Close)

	fake := testutil.NewFakeClientFactory()
	clients := &restConfigClientFactory{fake, &rest.Config{Host: server.URL}}
	log := logrus.New()

	// The cluster doesn't know about the removed API yet, so the update is fine.
	// This populates the client factory's cached discovery client.
	require.NoError(t, CanUpdate(t.Context(), log, clients, "v99.99.99"))

	// Now the CRD gets created.
	fake.DynamicClient.Resources = append(fake.DynamicClient.Resources, &metav1.APIResourceList{
		GroupVersion: removedGroup + "/" + removedVersion,
		APIResources: []metav1.APIResource{{
			Name: removedResource, Kind: removedKind, Namespaced: false,
			Verbs: metav1.Verbs{"get", "list"},
		}},
	})

	err := CanUpdate(t.Context(), log, clients, "v99.99.99")
	assert.ErrorContains(t, err,
		removedResource+"."+removedGroup+" "+removedVersion+" has been removed in Kubernetes v99.99.99, but there are 1 such resources in the cluster")
}
