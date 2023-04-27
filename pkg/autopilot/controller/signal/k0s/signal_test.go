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

package k0s

import (
	"context"
	"fmt"
	"testing"
	"time"

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
										"k0supdate": {
											"version": "v99.99.99",
											"url": "https://k0s.example.com/downloads/k0s-v99.99.99",
											"timestamp": "1980-01-01T00:00:00Z",
											"sha256": "0000000000000000000000000000000000000000000000000000000000000000"
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
											"version": "v99.99.99",
											"url": "https://k0s.example.com/downloads/k0s-v99.99.99",
											"timestamp": "1980-01-01T00:00:00Z",
											"sha256": "0000000000000000000000000000000000000000000000000000000000000000"
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
											"version": "v99.99.99",
											"url": "https://k0s.example.com/downloads/k0s-v99.99.99",
											"timestamp": "1980-01-01T00:00:00Z",
											"sha256": "0000000000000000000000000000000000000000000000000000000000000000"
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
											"version": "v99.99.99",
											"url": "https://k0s.example.com/downloads/k0s-v99.99.99",
											"timestamp": "1980-01-01T00:00:00Z",
											"sha256": "0000000000000000000000000000000000000000000000000000000000000000"
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
											"version": "v99.99.99",
											"url": "https://k0s.example.com/downloads/k0s-v99.99.99",
											"timestamp": "1980-01-01T00:00:00Z",
											"sha256": "0000000000000000000000000000000000000000000000000000000000000000"
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

	pred := signalControllerEventFilter("node0", func(err error) bool {
		return false
	})

	for idx, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.success, pred.Update(test.event), fmt.Sprintf("Failed in #%d '%s'", idx, test.name))
		})
	}
}

// TestSignalControllerSameVersion ensures that when requesting the same k0s version, the signaling
// response transitions to 'Completed'
func TestSignalControllerSameVersion(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())

	commonObjectMeta := metav1.ObjectMeta{
		Name: "foo",
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
					}
				}
			`,
		},
	}

	var tests = []struct {
		name     string
		objects  []crcli.Object
		delegate apdel.ControllerDelegate
	}{
		{
			"ControlNode",
			[]crcli.Object{
				&apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: commonObjectMeta,
				},
			},
			apdel.ControlNodeControllerDelegate(),
		},
		{
			"Node",
			[]crcli.Object{
				&v1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: commonObjectMeta,
				},
			},
			apdel.NodeControllerDelegate(),
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
				&signalControllerHandler{
					timeout:           SignalResponseProcessingTimeout,
					k0sVersionHandler: echoedK0sVersionHandler("v99.99.99"),
				},
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
			assert.NotNil(t, signalData.Status)

			if signalData.Status != nil {
				assert.Equal(t, apsigcomm.Completed, signalData.Status.Status)
			}
		})
	}
}

// TestSignalControllerSameVersionForceUpdate ensures that when requesting the same k0s version when
// 'forceupdate' is provided, the signaling response transitions to 'Completed'
func TestSignalControllerSameVersionForceUpdate(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())

	commonObjectMeta := metav1.ObjectMeta{
		Name: "foo",
		Annotations: map[string]string{
			"k0sproject.io/autopilot-signal-version": "v2",
			"k0sproject.io/autopilot-signal-data": `
				{
					"planId":"abc123",
					"created":"now",
					"command": {
						"id": 123,
						"k0supdate": {
							"forceupdate": true,
							"version": "v99.99.99",
							"url": "https://k0s.example.com/downloads/k0s-v99.99.99",
							"timestamp": "1980-01-01T00:00:00Z",
							"sha256": "0000000000000000000000000000000000000000000000000000000000000000"
		}
					}
				}
			`,
		},
	}

	var tests = []struct {
		name     string
		objects  []crcli.Object
		delegate apdel.ControllerDelegate
	}{
		{
			"ControlNode",
			[]crcli.Object{
				&apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: commonObjectMeta,
				},
			},
			apdel.ControlNodeControllerDelegate(),
		},
		{
			"Node",
			[]crcli.Object{
				&v1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: commonObjectMeta,
				},
			},
			apdel.NodeControllerDelegate(),
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
				&signalControllerHandler{
					timeout:           SignalResponseProcessingTimeout,
					k0sVersionHandler: echoedK0sVersionHandler("v99.99.99"),
				},
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
			assert.NotNil(t, signalData.Status)

			if signalData.Status != nil {
				assert.Equal(t, Downloading, signalData.Status.Status)
			}
		})
	}
}

// TestSignalControllerNewVersion ensures that when requesting a new k0s version, the signaling
// response transitions to 'Downloading' for all signal node implementations.
func TestSignalControllerNewVersion(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())

	commonObjectMeta := metav1.ObjectMeta{
		Name: "foo",
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
					}
				}
			`,
		},
	}

	var tests = []struct {
		name     string
		objects  []crcli.Object
		delegate apdel.ControllerDelegate
	}{
		{
			"ControlNode",
			[]crcli.Object{
				&apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: commonObjectMeta,
				},
			},
			apdel.ControlNodeControllerDelegate(),
		},
		{
			"Node",
			[]crcli.Object{
				&v1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: commonObjectMeta,
				},
			},
			apdel.NodeControllerDelegate(),
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
				&signalControllerHandler{
					timeout:           SignalResponseProcessingTimeout,
					k0sVersionHandler: echoedK0sVersionHandler("v99.99.99+k0s.NEW"),
				},
			)

			req := crrec.Request{
				NamespacedName: types.NamespacedName{Name: "foo"},
			}

			// Reconciling a signaling request that requests a version that matches the current installed version
			// should jump immediately to 'Downloading'.
			_, err := c.Reconcile(context.TODO(), req)
			assert.NoError(t, err)

			// Re-fetch the signal node again to confirm the status update
			signalNode := test.delegate.CreateObject()
			assert.NoError(t, client.Get(context.TODO(), req.NamespacedName, signalNode))

			var signalData apsigv2.SignalData
			err = signalData.Unmarshal(signalNode.GetAnnotations())

			assert.NoError(t, err)
			assert.NotNil(t, signalData)
			assert.NotNil(t, signalData.Status)

			if signalData.Status != nil {
				assert.Equal(t, Downloading, signalData.Status.Status)
			}
		})
	}
}

// TestCheckExpiredInvalid ensures that certain commands that are bound by expiration actually expire.
func TestCheckExpiredInvalid(t *testing.T) {
	var tests = []struct {
		name    string
		data    *apsigv2.SignalData
		timeout time.Duration
		expired bool
	}{
		// Ensures that a request that is currently 'Downloading', with a timestamp that hasn't
		// expired is considered 'still active', and its request will not be returned.
		{
			"Downloading in progress + not-expired",
			&apsigv2.SignalData{
				PlanID:  "id123",
				Created: "now",
				Command: apsigv2.Command{
					ID: new(int),
					K0sUpdate: &apsigv2.CommandK0sUpdate{
						Version: "v99.99.99",
						URL:     "https://k0s.example.com/downloads/k0s-v99.99.99",
						Sha256:  "0000000000000000000000000000000000000000000000000000000000000000",
					},
				},
				Status: &apsigv2.Status{
					Status:    "Downloading",
					Timestamp: "2021-10-20T19:09:11Z",
				},
			},
			1000000 * time.Hour,
			false,
		},

		// Ensures that a request that is currently 'ApplyingUpdate', with a timestamp that has
		// exceeded our expected timeout results in the request being returned.
		{
			"Processing in ApplyingUpdate + expired",
			&apsigv2.SignalData{
				PlanID:  "id123",
				Created: "now",
				Command: apsigv2.Command{
					ID: new(int),
					K0sUpdate: &apsigv2.CommandK0sUpdate{
						Version: "v99.99.99",
						URL:     "https://k0s.example.com/downloads/k0s-v99.99.99",
						Sha256:  "0000000000000000000000000000000000000000000000000000000000000000",
					},
				},
				Status: &apsigv2.Status{
					Status:    "ApplyingUpdate",
					Timestamp: "2021-10-20T19:09:11Z",
				},
			},
			SignalResponseProcessingTimeout,
			true,
		},

		// Ensures that a response that has an invalid RFC3339 timestamp value results in the request
		// being returned again for reprocessing.
		{
			"Invalid status timestamp",
			&apsigv2.SignalData{
				PlanID:  "id123",
				Created: "now",
				Command: apsigv2.Command{
					ID: new(int),
					K0sUpdate: &apsigv2.CommandK0sUpdate{
						Version: "v99.99.99",
						URL:     "https://k0s.example.com/downloads/k0s-v99.99.99",
						Sha256:  "0000000000000000000000000000000000000000000000000000000000000000",
					},
				},
				Status: &apsigv2.Status{
					Status:    "ApplyingUpdate",
					Timestamp: "2021-10-20INVALIDNOWT19:09:11Z",
				},
			},
			SignalResponseProcessingTimeout,
			true,
		},
	}

	logger := logrus.NewEntry(logrus.StandardLogger())

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expired := checkExpiredInvalid(logger, test.data, test.timeout)
			assert.Equal(t, test.expired, expired, fmt.Sprintf("Failure in '%s'", test.name))
		})
	}
}

// echoedK0sVersionHandler is a `k0sVersionHandlerFunc` that returns the version
// that you provide to it.
func echoedK0sVersionHandler(version string) k0sVersionHandlerFunc {
	return func() (string, error) {
		return version, nil
	}
}
