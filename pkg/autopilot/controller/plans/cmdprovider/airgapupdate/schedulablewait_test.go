// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package airgapupdate

import (
	"testing"

	"github.com/k0sproject/k0s/internal/testutil"
	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	apsigcomm "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"
	apscheme "github.com/k0sproject/k0s/pkg/client/clientset/scheme"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func signalNodeStatusDataAnnotations(sd apsigv2.SignalData) map[string]string {
	data := make(map[string]string)
	_ = sd.Marshal(data)

	return data
}

// TestSchedulableWait runs through a table of plans, ensuring that the plan will move
// to `Schedulable` only under certain conditions.
func TestSchedulableWait(t *testing.T) {
	var tests = []struct {
		name                      string
		objects                   []crcli.Object
		command                   apv1beta2.PlanCommand
		status                    apv1beta2.PlanCommandStatus
		expectedNextState         apv1beta2.PlanStateType
		expectedRetry             bool
		expectedPlanStatusWorkers []apv1beta2.PlanCommandTargetStatus
	}{
		// Worker-only tests

		// Ensures that if all the worker nodes are marked as complete, then the evaluation done in 'SchedulableWait'
		// should transition the command to 'Completed'.
		{
			"WorkersOnlyAllNodesCompleted",
			[]crcli.Object{},
			apv1beta2.PlanCommand{
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdate{},
			},
			apv1beta2.PlanCommandStatus{
				State: appc.PlanSchedulableWait,
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdateStatus{
					Workers: []apv1beta2.PlanCommandTargetStatus{
						apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalCompleted),
						apv1beta2.NewPlanCommandTargetStatus("worker1", appc.SignalCompleted),
					},
				},
			},
			appc.PlanCompleted,
			false,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalCompleted),
				apv1beta2.NewPlanCommandTargetStatus("worker1", appc.SignalCompleted),
			},
		},

		// Ensures that if a node has been sent a signal, the state of the command should stay in
		// 'SchedulableWait', and request a retry.
		{
			"WorkersOnlySignalingSent",
			[]crcli.Object{},
			apv1beta2.PlanCommand{
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdate{},
			},
			apv1beta2.PlanCommandStatus{
				State: appc.PlanSchedulableWait,
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdateStatus{
					Workers: []apv1beta2.PlanCommandTargetStatus{
						apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalSent),
						apv1beta2.NewPlanCommandTargetStatus("worker1", appc.SignalCompleted),
					},
				},
			},
			appc.PlanSchedulableWait,
			true,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalSent),
				apv1beta2.NewPlanCommandTargetStatus("worker1", appc.SignalCompleted),
			},
		},

		// Ensures that if any of the nodes are in 'PendingSignal', they can be scheduled and the command
		// status transitions to 'Schedulable'
		{
			"WorkersOnlyPendingSignal",
			[]crcli.Object{},
			apv1beta2.PlanCommand{
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdate{
					Workers: apv1beta2.PlanCommandTarget{
						Limits: apv1beta2.PlanCommandTargetLimits{
							Concurrent: 1,
						},
					},
				},
			},
			apv1beta2.PlanCommandStatus{
				State: appc.PlanSchedulableWait,
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdateStatus{
					Workers: []apv1beta2.PlanCommandTargetStatus{
						apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalPending),
						apv1beta2.NewPlanCommandTargetStatus("worker1", appc.SignalCompleted),
					},
				},
			},
			appc.PlanSchedulable,
			false,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalPending),
				apv1beta2.NewPlanCommandTargetStatus("worker1", appc.SignalCompleted),
			},
		},

		// Ensures that if worker concurrency is == 2 and only one worker is considered pending, then
		// the status transitions to 'Schedulable'
		{
			"WorkersOnlyBatch2OneAvailable",
			[]crcli.Object{},
			apv1beta2.PlanCommand{
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdate{
					Workers: apv1beta2.PlanCommandTarget{
						Limits: apv1beta2.PlanCommandTargetLimits{
							Concurrent: 2,
						},
					},
				},
			},
			apv1beta2.PlanCommandStatus{
				State: appc.PlanSchedulableWait,
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdateStatus{
					Workers: []apv1beta2.PlanCommandTargetStatus{
						apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalCompleted),
						apv1beta2.NewPlanCommandTargetStatus("worker1", appc.SignalPending),
						apv1beta2.NewPlanCommandTargetStatus("worker2", appc.SignalCompleted),
					},
				},
			},
			appc.PlanSchedulable,
			false,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalCompleted),
				apv1beta2.NewPlanCommandTargetStatus("worker1", appc.SignalPending),
				apv1beta2.NewPlanCommandTargetStatus("worker2", appc.SignalCompleted),
			},
		},

		// Ensures that if a signal node status is different than what is known by
		// the PlanCommandTargetStatus, the updated status is recorded.
		{
			"SignalNodeStatusSync",
			[]crcli.Object{
				&v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker0",
						Annotations: signalNodeStatusDataAnnotations(
							apsigv2.SignalData{
								PlanID:  "id123",
								Created: "now",
								Command: apsigv2.Command{
									AirgapUpdate: &apsigv2.CommandAirgapUpdate{
										URL:     "https://foo.bar.baz/download.tar.gz",
										Version: "v1.2.3",
									},
								},
								Status: &apsigv2.Status{
									Status:    apsigcomm.Completed,
									Timestamp: "now",
								},
							},
						),
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
				},
			},
			apv1beta2.PlanCommand{
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdate{},
			},
			apv1beta2.PlanCommandStatus{
				State: appc.PlanSchedulableWait,
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdateStatus{
					Workers: []apv1beta2.PlanCommandTargetStatus{
						apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalSent),
					},
				},
			},
			appc.PlanSchedulableWait,
			true,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalSent),
			},
		},

		// Covers the scenario of a v1.Node that contains autopilot state that indicates that
		// an update has completed, with a different plan ID from the test data. This should
		// result in the v1.Node autopilot state NOT getting reconciled as current, and ignored.
		{
			"WorkerNoPlanIDMatch",
			[]crcli.Object{
				&v1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker0",
						Annotations: map[string]string{
							"k0sproject.io/autopilot-signal-version": apsigv2.Version,
							"k0sproject.io/autopilot-signal-data":    `{"planId":"id999","created":"2022-07-01T00:56:19Z","command":{"id":0,"airgapupdate":{"url":"http://localhost/dist/k0s","version":"v0.0.0","forceupdate":true}},"status":{"status":"Completed","timestamp":"2022-07-01T00:56:27Z"}}`,
						},
					},
				},
			},
			apv1beta2.PlanCommand{
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdate{
					Workers: apv1beta2.PlanCommandTarget{
						Limits: apv1beta2.PlanCommandTargetLimits{
							Concurrent: 1,
						},
					},
				},
			},
			apv1beta2.PlanCommandStatus{
				State: appc.PlanSchedulableWait,
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdateStatus{
					Workers: []apv1beta2.PlanCommandTargetStatus{
						apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalPending),
					},
				},
			},
			appc.PlanSchedulable,
			false,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalPending),
			},
		},

		// Covers the scenario of a v1.Node that contains autopilot state that indicates that
		// an update has completed, with the same plan ID as the test data. This should result
		// in the v1.Node autopilot state getting reconciled as current.
		{
			"WorkerPlanIDMatch",
			[]crcli.Object{
				&v1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker0",
						Annotations: map[string]string{
							"k0sproject.io/autopilot-signal-version": apsigv2.Version,
							"k0sproject.io/autopilot-signal-data":    `{"planId":"id123","created":"2022-07-01T00:56:19Z","command":{"id":0,"airgapupdate":{"url":"http://localhost/dist/k0s","version":"v0.0.0","forceupdate":true}},"status":{"status":"Completed","timestamp":"2022-07-01T00:56:27Z"}}`,
						},
					},
				},
			},
			apv1beta2.PlanCommand{
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdate{
					Workers: apv1beta2.PlanCommandTarget{
						Limits: apv1beta2.PlanCommandTargetLimits{
							Concurrent: 1,
						},
					},
				},
			},
			apv1beta2.PlanCommandStatus{
				State: appc.PlanSchedulableWait,
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdateStatus{
					Workers: []apv1beta2.PlanCommandTargetStatus{
						apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalPending),
					},
				},
			},
			appc.PlanCompleted,
			false,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalCompleted),
			},
		},

		// Cover the scenario where a node fails to apply an update, and that the failure
		// is propagated back up to the plan state, resulting in the plan terminating.
		{
			"SignalNodeApplyFailure",
			[]crcli.Object{
				&v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker0",
						Annotations: signalNodeStatusDataAnnotations(
							apsigv2.SignalData{
								PlanID:  "id123",
								Created: "now",
								Command: apsigv2.Command{
									ID: new(int),
									AirgapUpdate: &apsigv2.CommandAirgapUpdate{
										URL:     "https://foo.bar.baz/download.tar.gz",
										Version: "v1.2.3",
									},
								},
								Status: &apsigv2.Status{
									Status:    apsigcomm.Failed,
									Timestamp: "now",
								},
							},
						),
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
				},
			},
			apv1beta2.PlanCommand{
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdate{},
			},
			apv1beta2.PlanCommandStatus{
				State: appc.PlanSchedulableWait,
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdateStatus{
					Workers: []apv1beta2.PlanCommandTargetStatus{
						apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalSent),
					},
				},
			},
			appc.PlanApplyFailed,
			false,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalApplyFailed),
			},
		},
	}

	scheme := runtime.NewScheme()
	assert.NoError(t, apscheme.AddToScheme(scheme))
	assert.NoError(t, v1.AddToScheme(scheme))

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := crfake.NewClientBuilder().WithObjects(test.objects...).WithScheme(scheme).Build()

			provider := NewAirgapUpdatePlanCommandProvider(
				logrus.NewEntry(logrus.StandardLogger()),
				client,
				map[string]apdel.ControllerDelegate{
					"controller": apdel.ControlNodeControllerDelegate(),
					"worker":     apdel.NodeControllerDelegate(),
				},
				testutil.NewFakeClientFactory(),
				[]string{},
			)

			ctx := t.Context()
			nextState, retry, err := provider.SchedulableWait(ctx, "id123", test.command, &test.status)

			assert.Equal(t, test.expectedNextState, nextState)
			assert.Equal(t, test.expectedRetry, retry)
			assert.NoError(t, err)
			assert.True(t, cmp.Equal(test.expectedPlanStatusWorkers, test.status.AirgapUpdate.Workers, cmpopts.IgnoreFields(apv1beta2.PlanCommandTargetStatus{}, "LastUpdatedTimestamp")))
		})
	}
}
