// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apdl "github.com/k0sproject/k0s/pkg/autopilot/download"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"
	apscheme "github.com/k0sproject/k0s/pkg/client/clientset/scheme"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	cr "sigs.k8s.io/controller-runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type testManifestBuilder struct {
	url         string
	downloadDir string
}

func (b testManifestBuilder) Build(_ crcli.Object, _ apsigv2.SignalData) (DownloadManifest, error) {
	return DownloadManifest{
		Config: apdl.Config{
			URL:         b.url,
			DownloadDir: b.downloadDir,
		},
		SuccessState: "Completed",
	}, nil
}

// TestDownloadControllerWritesSignalErrorOnFailure verifies that when a download fails,
// the SignalErrorAnnotation is written to both ControlNode and Node objects and persisted
// via client.Update.
func TestDownloadControllerWritesSignalErrorOnFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	commandID := 0

	tests := []struct {
		name     string
		delegate apdel.ControllerDelegate
		obj      crcli.Object
		nodeName string
		getObj   func(client crcli.Client) (crcli.Object, error)
	}{
		{
			name:     "ControlNode",
			delegate: apdel.ControlNodeControllerDelegate(),
			obj: &apv1beta2.ControlNode{
				TypeMeta:   metav1.TypeMeta{Kind: "ControlNode", APIVersion: "autopilot.k0sproject.io/v1beta2"},
				ObjectMeta: metav1.ObjectMeta{Name: "controller0"},
			},
			nodeName: "controller0",
			getObj: func(client crcli.Client) (crcli.Object, error) {
				cn := &apv1beta2.ControlNode{}
				return cn, client.Get(context.Background(), types.NamespacedName{Name: "controller0"}, cn)
			},
		},
		{
			name:     "Node",
			delegate: apdel.NodeControllerDelegate(),
			obj: &corev1.Node{
				TypeMeta:   metav1.TypeMeta{Kind: "Node", APIVersion: "v1"},
				ObjectMeta: metav1.ObjectMeta{Name: "worker0"},
			},
			nodeName: "worker0",
			getObj: func(client crcli.Client) (crcli.Object, error) {
				node := &corev1.Node{}
				return node, client.Get(context.Background(), types.NamespacedName{Name: "worker0"}, node)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build signal data annotations pointing at the failing server.
			signalData := apsigv2.SignalData{
				PlanID:  "testplan123",
				Created: "now",
				Command: apsigv2.Command{
					ID: &commandID,
					K0sUpdate: &apsigv2.CommandK0sUpdate{
						URL:     srv.URL + "/k0s",
						Version: "v1.0.0",
					},
				},
				Status: apsigv2.NewStatus(Downloading),
			}
			annotations := make(map[string]string)
			require.NoError(t, signalData.Marshal(annotations))
			tt.obj.SetAnnotations(annotations)

			scheme := apimruntime.NewScheme()
			require.NoError(t, apscheme.AddToScheme(scheme))
			require.NoError(t, corev1.AddToScheme(scheme))

			client := crfake.NewClientBuilder().
				WithObjects(tt.obj).
				WithScheme(scheme).
				Build()

			controller := NewDownloadController(
				logrus.NewEntry(logrus.New()),
				client,
				tt.delegate,
				testManifestBuilder{url: srv.URL + "/k0s", downloadDir: t.TempDir()},
			)

			_, err := controller.Reconcile(context.Background(), cr.Request{
				NamespacedName: types.NamespacedName{Name: tt.nodeName},
			})
			require.NoError(t, err)

			updated, err := tt.getObj(client)
			require.NoError(t, err)

			raw, ok := updated.GetAnnotations()[apdel.SignalErrorAnnotation]
			require.True(t, ok, "SignalErrorAnnotation should be set on %s", tt.name)

			var se apdel.SignalError
			require.NoError(t, json.Unmarshal([]byte(raw), &se))
			assert.Equal(t, "testplan123", se.PlanID)
			assert.Equal(t, FailedDownload, se.Reason)
			assert.NotEmpty(t, se.Message)
		})
	}
}
