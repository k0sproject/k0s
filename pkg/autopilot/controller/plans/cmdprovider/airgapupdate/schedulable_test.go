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

package airgapupdate

import (
	"context"
	"testing"

	aptcomm "github.com/k0sproject/k0s/inttest/autopilot/common"
	apv1beta2 "github.com/k0sproject/k0s/pkg/autopilot/apis/autopilot.k0sproject.io/v1beta2"
	apscheme "github.com/k0sproject/k0s/pkg/autopilot/apis/autopilot.k0sproject.io/v1beta2/clientset/scheme"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
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
		name                      string
		objects                   []crcli.Object
		command                   apv1beta2.PlanCommand
		status                    apv1beta2.PlanCommandStatus
		expectedNextState         apv1beta2.PlanStateType
		expectedRetry             bool
		expectedError             bool
		expectedPlanStatusWorkers []apv1beta2.PlanCommandTargetStatus
	}{
		// Ensures that if a controller is completed, no additional execution will occur.
		{
			"WorkerCompletedNoExecute",
			[]crcli.Object{
				&v1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:        "worker0",
						Annotations: aptcomm.DefaultNodeLabels(),
					},
				},
			},
			apv1beta2.PlanCommand{
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdate{
					Version: "v1.22.2+k0s.1",
					Platforms: apv1beta2.PlanPlatformResourceURLMap{
						"linux-amd64": {
							URL:    "https://github.com/k0sproject/k0s/releases/download/v1.22.2%2Bk0s.1/k0s-airgap-bundle-v1.22.2+k0s.1-amd64",
							Sha256: "7e96a9827360ff0184faacdbaa82cb8db318532a89acc8f4ec1bdac244757d23",
						},
					},
					Workers: apv1beta2.PlanCommandTarget{
						Discovery: apv1beta2.PlanCommandTargetDiscovery{
							Static: &apv1beta2.PlanCommandTargetDiscoveryStatic{
								Nodes: []string{"worker0"},
							},
						},
					},
				},
			},
			apv1beta2.PlanCommandStatus{
				State: appc.PlanSchedulable,
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdateStatus{
					Workers: []apv1beta2.PlanCommandTargetStatus{
						apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalCompleted),
					},
				},
			},
			appc.PlanCompleted,
			false,
			false,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalCompleted),
			},
		},

		// Ensures that a signal node can be sent a signal, and individually transition
		// to 'SignalingSent'.
		{
			"HappyMoveToSchedulableWait",
			[]crcli.Object{
				&v1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:   "worker0",
						Labels: aptcomm.DefaultNodeLabels(),
					},
				},
			},
			apv1beta2.PlanCommand{
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdate{
					Version: "v1.22.2+k0s.1",
					Platforms: apv1beta2.PlanPlatformResourceURLMap{
						"linux-amd64": {
							URL:    "https://github.com/k0sproject/k0s/releases/download/v1.22.2%2Bk0s.1/k0s-airgap-bundle-v1.22.2+k0s.1-amd64",
							Sha256: "7e96a9827360ff0184faacdbaa82cb8db318532a89acc8f4ec1bdac244757d23",
						},
					},
					Workers: apv1beta2.PlanCommandTarget{
						Discovery: apv1beta2.PlanCommandTargetDiscovery{
							Static: &apv1beta2.PlanCommandTargetDiscoveryStatic{
								Nodes: []string{"worker0"},
							},
						},
					},
				},
			},
			apv1beta2.PlanCommandStatus{
				State: appc.PlanSchedulable,
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdateStatus{
					Workers: []apv1beta2.PlanCommandTargetStatus{
						apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalPending),
					},
				},
			},
			appc.PlanSchedulableWait,
			false,
			false,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalSent),
			},
		},
	}

	scheme := apimruntime.NewScheme()
	apscheme.AddToScheme(scheme)
	v1.AddToScheme(scheme)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := crfake.NewClientBuilder().WithObjects(test.objects...).WithScheme(scheme).Build()

			provider := NewAirgapUpdatePlanCommandProvider(
				logrus.NewEntry(logrus.StandardLogger()),
				client,
				map[string]apdel.ControllerDelegate{
					"worker": apdel.NodeControllerDelegate(),
				},
				[]string{},
			)

			ctx := context.TODO()
			nextState, retry, err := provider.Schedulable(ctx, test.command, &test.status)

			assert.Equal(t, test.expectedNextState, nextState)
			assert.Equal(t, test.expectedRetry, retry)
			assert.Equal(t, test.expectedError, err != nil, "Unexpected error: %v", err)
			assert.True(t, cmp.Equal(test.expectedPlanStatusWorkers, test.status.AirgapUpdate.Workers, cmpopts.IgnoreFields(apv1beta2.PlanCommandTargetStatus{}, "LastUpdatedTimestamp")))
		})
	}
}
