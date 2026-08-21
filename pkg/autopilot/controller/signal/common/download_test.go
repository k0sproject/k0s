// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apdl "github.com/k0sproject/k0s/pkg/autopilot/download"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"
	apscheme "github.com/k0sproject/k0s/pkg/client/clientset/scheme"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cr "sigs.k8s.io/controller-runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

type testManifestBuilder struct {
	downloadDir  string
	url          string
	successState string
}

func (b *testManifestBuilder) Build(_ crcli.Object, _ apsigv2.SignalData) (DownloadManifest, error) {
	return DownloadManifest{
		Config: apdl.Config{
			URL:         b.url,
			DownloadDir: b.downloadDir,
			Filename:    "k0s.tmp",
		},
		SuccessState: b.successState,
	}, nil
}

// TestDownloadControllerConflict ensures that a signal node that gets modified
// while its update is being downloaded doesn't cause the download to be
// repeated, and that the concurrent modification is retained.
func TestDownloadControllerConflict(t *testing.T) {
	var downloads atomic.Uint32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		downloads.Add(1)
		_, _ = w.Write([]byte("some k0s binary"))
	}))
	t.Cleanup(server.Close)

	scheme := apimruntime.NewScheme()
	require.NoError(t, apscheme.AddToScheme(scheme))

	signalData := apsigv2.SignalData{
		PlanID:  "abc123",
		Created: "now",
		Command: apsigv2.Command{
			ID: new(int),
			K0sUpdate: &apsigv2.CommandK0sUpdate{
				URL:     server.URL,
				Version: "v1.2.3",
			},
		},
		Status: apsigv2.NewStatus(Downloading),
	}
	annotations := map[string]string{}
	require.NoError(t, signalData.Marshal(annotations))

	signalNode := &apv1beta2.ControlNode{
		ObjectMeta: metav1.ObjectMeta{Name: "controller0", Annotations: annotations},
	}

	// Modify the signal node behind the reconciler's back while it's updating
	// it, so that the first update attempt runs into a conflict.
	var conflictInjected atomic.Bool
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(signalNode).
		WithInterceptorFuncs(interceptor.Funcs{
			Update: func(ctx context.Context, client crcli.WithWatch, obj crcli.Object, opts ...crcli.UpdateOption) error {
				if conflictInjected.Swap(true) {
					return client.Update(ctx, obj, opts...)
				}

				var concurrent apv1beta2.ControlNode
				if err := client.Get(ctx, crcli.ObjectKeyFromObject(obj), &concurrent); err != nil {
					return err
				}
				concurrent.Annotations["test.k0sproject.io/touch"] = "touched"
				if err := client.Update(ctx, &concurrent); err != nil {
					return err
				}

				return apierrors.NewConflict(
					schema.GroupResource{Group: "autopilot.k0sproject.io", Resource: "controlnodes"},
					obj.GetName(), errors.New("injected conflict"),
				)
			},
		}).
		Build()

	underTest := NewDownloadController(
		logrus.NewEntry(logrus.StandardLogger()),
		client,
		apdel.ControlNodeControllerDelegate(),
		&testManifestBuilder{
			downloadDir:  t.TempDir(),
			url:          server.URL,
			successState: "Cordoning",
		},
	)

	_, err := underTest.Reconcile(t.Context(), cr.Request{
		NamespacedName: types.NamespacedName{Name: signalNode.Name},
	})
	require.NoError(t, err)
	assert.True(t, conflictInjected.Load(), "Conflict hasn't been injected")
	assert.Equal(t, uint32(1), downloads.Load(), "Download has been repeated")

	var updated apv1beta2.ControlNode
	require.NoError(t, client.Get(t.Context(), crcli.ObjectKeyFromObject(signalNode), &updated))

	var updatedSignalData apsigv2.SignalData
	require.NoError(t, updatedSignalData.Unmarshal(updated.Annotations))
	if assert.NotNil(t, updatedSignalData.Status) {
		assert.Equal(t, "Cordoning", updatedSignalData.Status.Status)
	}
	assert.Equal(t, "touched", updated.Annotations["test.k0sproject.io/touch"],
		"Concurrent modification has been lost")
}
