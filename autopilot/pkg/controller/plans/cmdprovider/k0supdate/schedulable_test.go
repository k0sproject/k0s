// Copyright 2022 k0s authors
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

package k0supdate

import (
	"context"
	"testing"

	aptcomm "github.com/k0sproject/autopilot/inttest/common"
	apv1beta2 "github.com/k0sproject/autopilot/pkg/apis/autopilot.k0sproject.io/v1beta2"
	apscheme "github.com/k0sproject/autopilot/pkg/apis/autopilot.k0sproject.io/v1beta2/clientset/scheme"
	apdel "github.com/k0sproject/autopilot/pkg/controller/delegate"
	appc "github.com/k0sproject/autopilot/pkg/controller/plans/core"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimruntime "k8s.io/apimachinery/pkg/runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestSchedulable tests the reconcile function of the `scheduleable` controller.
// This ensures that the proper signaling data is sent to control nodes, and that reconciliation
// ends when proper conditions are met.
func TestSchedulable(t *testing.T) {
	var tests = []struct {
		name                          string
		objects                       []crcli.Object
		command                       apv1beta2.PlanCommand
		status                        apv1beta2.PlanCommandStatus
		expectedNextState             apv1beta2.PlanStateType
		expectedRetry                 bool
		expectedError                 bool
		expectedPlanStatusControllers []apv1beta2.PlanCommandTargetStatus
		expectedPlanStatusWorkers     []apv1beta2.PlanCommandTargetStatus
	}{
		// Ensures that if a controller is completed, no additional execution will occur.
		{
			"ControllerCompletedNoExecute",
			[]crcli.Object{
				&apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:        "controller0",
						Annotations: aptcomm.DefaultNodeLabels(),
					},
				},
			},
			apv1beta2.PlanCommand{
				K0sUpdate: &apv1beta2.PlanCommandK0sUpdate{
					Version: "v1.22.2+k0s.1",
					Platforms: apv1beta2.PlanPlatformResourceURLMap{
						"linux-amd64": {
							URL:    "https://github.com/k0sproject/k0s/releases/download/v1.22.2%2Bk0s.1/k0s-v1.22.2+k0s.1-amd64",
							Sha256: "08840f0883d704e70a9119f1a95906c14fa75e91529e2daca27a081001a96fdb",
						},
					},
					Targets: apv1beta2.PlanCommandTargets{
						Controllers: apv1beta2.PlanCommandTarget{
							Discovery: apv1beta2.PlanCommandTargetDiscovery{
								Static: &apv1beta2.PlanCommandTargetDiscoveryStatic{
									Nodes: []string{"controller0"},
								},
							},
						},
					},
				},
			},
			apv1beta2.PlanCommandStatus{
				State: appc.PlanSchedulable,
				K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{
					Controllers: []apv1beta2.PlanCommandTargetStatus{
						apv1beta2.NewPlanCommandTargetStatus("controller0", appc.SignalCompleted),
					},
				},
			},
			appc.PlanCompleted,
			false,
			false,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("controller0", appc.SignalCompleted),
			},
			nil,
		},

		// Ensures that a signal node can be sent a signal, and individually transition
		// to 'SignalingSent'.
		{
			"HappyMoveToSchedulableWait",
			[]crcli.Object{
				&apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:   "controller0",
						Labels: aptcomm.DefaultNodeLabels(),
					},
				},
			},
			apv1beta2.PlanCommand{
				K0sUpdate: &apv1beta2.PlanCommandK0sUpdate{
					Version: "v1.22.2+k0s.1",
					Platforms: apv1beta2.PlanPlatformResourceURLMap{
						"linux-amd64": {
							URL:    "https://github.com/k0sproject/k0s/releases/download/v1.22.2%2Bk0s.1/k0s-v1.22.2+k0s.1-amd64",
							Sha256: "08840f0883d704e70a9119f1a95906c14fa75e91529e2daca27a081001a96fdb",
						},
					},
					Targets: apv1beta2.PlanCommandTargets{
						Controllers: apv1beta2.PlanCommandTarget{
							Discovery: apv1beta2.PlanCommandTargetDiscovery{
								Static: &apv1beta2.PlanCommandTargetDiscoveryStatic{
									Nodes: []string{"controller0"},
								},
							},
						},
					},
				},
			},
			apv1beta2.PlanCommandStatus{
				State: appc.PlanSchedulable,
				K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{
					Controllers: []apv1beta2.PlanCommandTargetStatus{
						apv1beta2.NewPlanCommandTargetStatus("controller0", appc.SignalPending),
					},
				},
			},
			appc.PlanSchedulableWait,
			false,
			false,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("controller0", appc.SignalSent),
			},
			nil,
		},
	}

	scheme := apimruntime.NewScheme()
	apscheme.AddToScheme(scheme)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := crfake.NewClientBuilder().WithObjects(test.objects...).WithScheme(scheme).Build()

			provider := NewK0sUpdatePlanCommandProvider(
				logrus.NewEntry(logrus.StandardLogger()),
				client,
				map[string]apdel.ControllerDelegate{
					"controller": apdel.ControlNodeControllerDelegate(),
					"worker":     apdel.NodeControllerDelegate(),
				},
				[]string{},
			)

			ctx := context.TODO()
			nextState, retry, err := provider.Schedulable(ctx, test.command, &test.status)

			assert.Equal(t, test.expectedNextState, nextState)
			assert.Equal(t, test.expectedRetry, retry)
			assert.Equal(t, test.expectedError, err != nil, "Unexpected error: %v", err)

			assert.True(t, cmp.Equal(test.expectedPlanStatusControllers, test.status.K0sUpdate.Controllers, cmpopts.IgnoreFields(apv1beta2.PlanCommandTargetStatus{}, "LastUpdatedTimestamp")))
			assert.True(t, cmp.Equal(test.expectedPlanStatusWorkers, test.status.K0sUpdate.Workers, cmpopts.IgnoreFields(apv1beta2.PlanCommandTargetStatus{}, "LastUpdatedTimestamp")))

		})
	}
}

// TestFindNextPendingRandom ensures that we can find a random PlanCommandTargetStatus
// that is in the `PendingStatus` state.
func TestFindNextPendingRandom(t *testing.T) {
	nodes := []apv1beta2.PlanCommandTargetStatus{
		apv1beta2.NewPlanCommandTargetStatus("aaa", appc.SignalCompleted),
		apv1beta2.NewPlanCommandTargetStatus("bbb", appc.SignalPending),
		apv1beta2.NewPlanCommandTargetStatus("ccc", appc.SignalCompleted),
		apv1beta2.NewPlanCommandTargetStatus("ddd", appc.SignalPending),
		apv1beta2.NewPlanCommandTargetStatus("eee", appc.SignalPending),
	}

	countMap := make(map[string]int)

	// Run through the random function a number of times and assert that
	// randomness is working. Its possible that one node can be chosen
	// 100% of the time, but its unlikely.

	for i := 0; i < 1000; i++ {
		node, err := findNextPendingRandom(nodes)
		assert.NoError(t, err)
		assert.NotNil(t, node)

		countMap[node.Name] = countMap[node.Name] + 1
	}

	assert.Contains(t, countMap, "bbb")
	assert.Contains(t, countMap, "ddd")
	assert.Contains(t, countMap, "eee")
	assert.NotEqual(t, 1000, countMap["bbb"])
	assert.NotEqual(t, 1000, countMap["ddd"])
	assert.NotEqual(t, 1000, countMap["eee"])
}
