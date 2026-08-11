//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package k0s

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"
	apscheme "github.com/k0sproject/k0s/pkg/client/clientset/scheme"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	crev "sigs.k8s.io/controller-runtime/pkg/event"
	crrec "sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type failFirstUpdateClient struct {
	crcli.Client
	updateAttempts int
}

func (c *failFirstUpdateClient) Update(ctx context.Context, obj crcli.Object, opts ...crcli.UpdateOption) error {
	c.updateAttempts++
	if c.updateAttempts == 1 {
		return errors.New("simulated update conflict")
	}
	return c.Client.Update(ctx, obj, opts...)
}

// TestApplyingUpdateEventFilter runs through a table of scenarios ensuring that
// the event-filtering for the 'applying-update' controller works accordingly.
func TestApplyingUpdateEventFilter(t *testing.T) {
	var tests = []struct {
		name    string
		event   crev.UpdateEvent
		success bool
	}{
		{
			"Happy",
			crev.UpdateEvent{
				ObjectOld: &apv1beta2.ControlNode{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
					},
				},
				ObjectNew: &apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node0",
						Annotations: map[string]string{
							"k0sproject.io/autopilot-signal-version": "v2",
							"k0sproject.io/autopilot-signal-data": `
								{
									"planId":"abc123",
									"created":"now",
									"command": {
										"id": 123,
										"k0supdate": {
											"version": "v99.99.99",
											"url": "https://k0s.example.com/downloads/k0s-v99.99.99",
											"timestamp": "1980-01-01T00:00:00Z",
											"sha256": "0000000000000000000000000000000000000000000000000000000000000000"
										}
									},
									"status": {
										"status": "ApplyingUpdate",
										"timestamp": "2022-06-22T12:21:54Z"
									}
								}
							`,
						},
					},
				},
			},
			true,
		},
		{
			"Wrong response",
			crev.UpdateEvent{
				ObjectOld: &apv1beta2.ControlNode{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
					},
				},
				ObjectNew: &apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node0",
						Annotations: map[string]string{
							"k0sproject.io/autopilot-signal-version": "v2",
							"k0sproject.io/autopilot-signal-data": `
								{
									"planId":"abc123",
									"created":"now",
									"command": {
										"id": 123,
										"k0supdate": {
											"version": "v99.99.99",
											"url": "https://k0s.example.com/downloads/k0s-v99.99.99",
											"timestamp": "1980-01-01T00:00:00Z",
											"sha256": "0000000000000000000000000000000000000000000000000000000000000000"
										}
									},
									"status": {
										"status": "Completed",
										"timestamp": "2022-06-22T12:21:54Z"
									}
								}
							`,
						},
					},
				},
			},
			false,
		},
		{
			"No change in annotations",
			crev.UpdateEvent{
				ObjectOld: &apv1beta2.ControlNode{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"k0sproject.io/autopilot-signal-version": "v2",
							"k0sproject.io/autopilot-signal-data": `
								{
									"planId":"abc123",
									"created":"now",
									"command": {
										"id": 123,
										"k0supdate": {
											"version": "v1.2.3",
											"url": "https://www.google.com/download.tar.gz",
											"timestamp": "2021-10-20T19:06:56Z",
											"sha256": "thisisthesha"
										}
									}
								}
							`,
						},
					},
				},
				ObjectNew: &apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node0",
						Annotations: map[string]string{
							"k0sproject.io/autopilot-signal-version": "v2",
							"k0sproject.io/autopilot-signal-data": `
								{
									"planId":"abc123",
									"created":"now",
									"command": {
										"id": 123,
										"k0supdate": {
											"version": "v1.2.3",
											"url": "https://www.google.com/download.tar.gz",
											"timestamp": "2021-10-20T19:06:56Z",
											"sha256": "thisisthesha"
										}
									}
									"status": {
										"status": "Completed",
										"timestamp": "2021-10-20T19:09:11Z"
									}
								}
							`,
						},
					},
				},
			},
			false,
		},
		{
			"Different hostname",
			crev.UpdateEvent{
				ObjectOld: &apv1beta2.ControlNode{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
					},
				},
				ObjectNew: &apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "nodeDIFFERENT",
						Annotations: map[string]string{
							"k0sproject.io/autopilot-signal-version": "v2",
							"k0sproject.io/autopilot-signal-data": `
								{
									"planId":"abc123",
									"created":"now",
									"command": {
										"id": 123,
										"k0supdate": {
											"version": "v1.2.3",
											"url": "https://www.google.com/download.tar.gz",
											"timestamp": "2021-10-20T19:06:56Z",
											"sha256": "thisisthesha"
										}
									}
									"status": {
										"status": "Completed",
										"timestamp": "2021-10-20T19:09:11Z"
									}
								}
							`,
						},
					},
				},
			},
			false,
		},
	}

	pred := applyingUpdateEventFilter("node0", func(err error) bool {
		return false
	})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.success, pred.Update(test.event))
		})
	}
}

func TestApplyingUpdateReconcileRetryAfterUpdateFailure(t *testing.T) {
	commandID := 123
	signalData := apsigv2.SignalData{
		PlanID:  "plan-1",
		Created: "2026-01-01T00:00:00Z",
		Command: apsigv2.Command{
			ID: &commandID,
			K0sUpdate: &apsigv2.CommandK0sUpdate{
				URL:     "https://updates.example.com/k0s",
				Version: "v1.2.3",
			},
		},
		Status: &apsigv2.Status{Status: ApplyingUpdate, Timestamp: "2026-01-01T00:00:00Z"},
	}

	annotations := map[string]string{}
	require.NoError(t, signalData.Marshal(annotations))

	signalNode := &apv1beta2.ControlNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "node0",
			Annotations: annotations,
		},
	}

	scheme := runtime.NewScheme()
	require.NoError(t, apscheme.AddToScheme(scheme))
	baseClient := crfake.NewClientBuilder().WithObjects(signalNode).WithScheme(scheme).Build()

	delegate := apdel.ControlNodeControllerDelegate()
	client := &failFirstUpdateClient{Client: baseClient}
	restartInitiated := false
	reconciler := &applyingUpdate{
		log:              logrus.NewEntry(logrus.StandardLogger()),
		client:           client,
		delegate:         delegate,
		k0sBinaryDir:     t.TempDir(),
		restartInitiated: func(apsigv2.SignalData) { restartInitiated = true },
	}

	updateFilenamePath := filepath.Join(reconciler.k0sBinaryDir, apconst.K0sTempFilename)
	k0sBinaryFilenamePath := filepath.Join(reconciler.k0sBinaryDir, "k0s")
	updateLinkFilenamePath := filepath.Join(reconciler.k0sBinaryDir, apconst.K0sTempLinkFilename)

	require.NoError(t, os.WriteFile(updateFilenamePath, []byte("new k0s binary"), 0755))
	require.NoError(t, os.WriteFile(k0sBinaryFilenamePath, []byte("old k0s binary"), 0755))

	req := crrec.Request{NamespacedName: types.NamespacedName{Name: "node0"}}

	_, err := reconciler.Reconcile(t.Context(), req)
	require.Error(t, err)

	// The hard link keeps k0s.tmp around even though k0s was replaced, so a
	// retry can replay the apply sequence without erroring on a missing file.
	_, err = os.Stat(updateFilenamePath)
	require.NoError(t, err)
	replacedBinary, err := os.ReadFile(k0sBinaryFilenamePath)
	require.NoError(t, err)
	assert.Equal(t, []byte("new k0s binary"), replacedBinary)
	_, err = os.Stat(updateLinkFilenamePath)
	assert.True(t, os.IsNotExist(err))

	_, err = reconciler.Reconcile(t.Context(), req)
	require.NoError(t, err)
	assert.True(t, restartInitiated)

	updatedNode := delegate.CreateObject()
	require.NoError(t, baseClient.Get(t.Context(), req.NamespacedName, updatedNode))
	var updatedData apsigv2.SignalData
	require.NoError(t, updatedData.Unmarshal(updatedNode.GetAnnotations()))
	require.NotNil(t, updatedData.Status)
	assert.Equal(t, Restart, updatedData.Status.Status)

	_, err = os.Stat(updateFilenamePath)
	assert.True(t, os.IsNotExist(err))
}
