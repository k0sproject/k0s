// Copyright 2021 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package airgap

import (
	"context"
	"fmt"
	"testing"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apsigcomm "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"
	apscheme "github.com/k0sproject/k0s/pkg/client/clientset/scheme"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	crev "sigs.k8s.io/controller-runtime/pkg/event"
	crrec "sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// TestSignalControllerEventFilter runs through a table of scenarios ensuring that
// the event-filtering for the 'signal' controller works accordingly.
func TestSignalControllerEventFilter(t *testing.T) {
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
										"airgapupdate": {
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
			},
			true,
		},
		{
			"Unexpected response",
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
											"version": "v1.2.3",
											"url": "https://www.google.com/download.tar.gz",
											"timestamp": "2021-10-20T19:06:56Z",
											"sha256": "thisisthesha"
										}
									},
									"status": {
										"status": "Completed",
										"timestamp": "2021-10-20T19:06:56Z"
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
								}
							`,
						},
					},
				},
			},
			false,
		},
	}

	pred := SignalControllerEventFilter("node0", func(err error) bool {
		return false
	})

	for idx, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.success, pred.Update(test.event), fmt.Sprintf("Failed in #%d '%s'", idx, test.name))
		})
	}
}

// TestHandle performs some basic tests on an airgap signal being received, and moved into 'Downloading'
func TestHandle(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())

	var tests = []struct {
		name                     string
		objects                  []crcli.Object
		delegate                 apdel.ControllerDelegate
		expectedSignalDataStatus string
	}{
		// A happy-path scenario where a Node has been signaled to apply an update.
		{
			"Node",
			[]crcli.Object{
				&v1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
						Annotations: map[string]string{
							"k0sproject.io/autopilot-signal-version": "v2",
							"k0sproject.io/autopilot-signal-data": `
								{
									"planId":"abc123",
									"created":"now",
									"command": {
										"id": 123,
										"airgapupdate": {
											"version": "v99.99.99",
											"url": "https://k0s.example.com/downloads/k0s-airgap-bundle-v99.99.99",
											"sha256": "0000000000000000000000000000000000000000000000000000000000000000"
										}
									},
									"status": {
										"status": "Downloading",
										"timestamp": "now"
									}
								}
							`,
						},
					},
				},
			},
			apdel.NodeControllerDelegate(),
			"Downloading",
		},

		// Ensure that a command that is completed is properly skipped
		{
			"NodeAirgapUpdateCommandCompleted",
			[]crcli.Object{
				&v1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
						Annotations: map[string]string{
							"k0sproject.io/autopilot-signal-version": "v2",
							"k0sproject.io/autopilot-signal-data": `
								{
									"planId":"abc123",
									"created":"now",
									"command": {
										"id": 123,
										"airgapupdate": {
											"version": "v99.99.99",
											"url": "https://k0s.example.com/downloads/k0s-airgap-bundle-v99.99.99",
											"sha256": "0000000000000000000000000000000000000000000000000000000000000000"
										}
									},
									"status": {
										"status": "Completed",
										"timestamp": "now"
									}
								}
							`,
						},
					},
				},
			},
			apdel.NodeControllerDelegate(),
			"Completed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			assert.NoError(t, apscheme.AddToScheme(scheme))
			assert.NoError(t, v1.AddToScheme(scheme))

			client := crfake.NewClientBuilder().WithObjects(test.objects...).WithScheme(scheme).Build()

			c := apsigcomm.NewSignalController(
				logger,
				client,
				test.delegate,
				&signalControllerHandler{},
			)

			req := crrec.Request{
				NamespacedName: types.NamespacedName{Name: "foo"},
			}

			// Reconciling a signaling request that requests a version that matches the current installed version
			// should jump immediately to 'Completed'.
			_, err := c.Reconcile(context.TODO(), req)
			assert.NoError(t, err)

			// Re-fetch the signal node again to confirm the status update
			signalNode := test.delegate.CreateObject()
			assert.NoError(t, client.Get(context.TODO(), req.NamespacedName, signalNode))

			var signalData apsigv2.SignalData
			err = signalData.Unmarshal(signalNode.GetAnnotations())

			assert.NoError(t, err)
			assert.Equal(t, test.expectedSignalDataStatus, signalData.Status.Status)
		})
	}
}
