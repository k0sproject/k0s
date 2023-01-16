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

	"github.com/k0sproject/k0s/internal/testutil"
	aptcomm "github.com/k0sproject/k0s/inttest/autopilot/common"
	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2"
	apscheme "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2/clientset/scheme"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestNewPlan covers the scenarios of different new plans that enter
// the reconciler, ensuring the proper status of each.
func TestNewPlan(t *testing.T) {
	var tests = []struct {
		name                      string
		objects                   []crcli.Object
		command                   apv1beta2.PlanCommand
		expectedNextState         apv1beta2.PlanStateType
		expectedPlanStatusWorkers []apv1beta2.PlanCommandTargetStatus
		excludedFromPlans         []string
	}{
		// A happy scenario that includes both a controller and worker, and the
		// plan successfully gets processed as 'newplan'
		{
			"HappyWorker",
			[]crcli.Object{
				&v1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:   "worker0",
						Labels: aptcomm.LinuxAMD64NodeLabels(),
					},
				},
			},
			apv1beta2.PlanCommand{
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdate{
					Platforms: apv1beta2.PlanPlatformResourceURLMap{
						"linux-amd64": {},
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
			appc.PlanSchedulableWait,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalPending),
			},
			[]string{},
		},

		// A scenario where a plan indicates that both a controller and worker node are present,
		// however on discovery its determined that the worker is missing.
		{
			"MissingWorker",
			[]crcli.Object{
				&v1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:   "workerMISSING",
						Labels: aptcomm.LinuxAMD64NodeLabels(),
					},
				},
			},
			apv1beta2.PlanCommand{
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdate{
					Platforms: apv1beta2.PlanPlatformResourceURLMap{
						"linux-amd64": {},
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
			appc.PlanIncompleteTargets,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalMissingNode),
			},
			[]string{},
		},

		// A scenario where a plan indicates a worker, however on discovery
		// the worker is running on a different platform/architecture.
		{
			"MissingWorkerWrongNodePlatform",
			[]crcli.Object{
				&apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:   "controller0",
						Labels: aptcomm.LinuxAMD64NodeLabels(),
					},
				},
				&v1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker0",
						Labels: map[string]string{
							v1.LabelOSStable:   "windows",
							v1.LabelArchStable: "amd64",
						},
					},
				},
			},
			apv1beta2.PlanCommand{
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdate{
					Platforms: apv1beta2.PlanPlatformResourceURLMap{
						"linux-amd64": {},
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
			appc.PlanIncompleteTargets,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalMissingPlatform),
			},
			[]string{},
		},

		// A scenario where a plan details a controller and worker, however workers
		// have been intentionally excluded/prohibited.
		{
			"ExcludedWorkers",
			[]crcli.Object{
				&v1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:   "worker0",
						Labels: aptcomm.LinuxAMD64NodeLabels(),
					},
				},
			},
			apv1beta2.PlanCommand{
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdate{
					Platforms: apv1beta2.PlanPlatformResourceURLMap{
						"linux-amd64": {},
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
			appc.PlanRestricted,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalPending),
			},
			[]string{"worker"},
		},
	}

	scheme := runtime.NewScheme()
	assert.NoError(t, apscheme.AddToScheme(scheme))
	assert.NoError(t, v1.AddToScheme(scheme))

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger := logrus.NewEntry(logrus.StandardLogger())
			client := crfake.NewClientBuilder().WithObjects(test.objects...).WithScheme(scheme).Build()

			provider := NewAirgapUpdatePlanCommandProvider(
				logger,
				client,
				map[string]apdel.ControllerDelegate{
					"worker": apdel.NodeControllerDelegate(),
				},
				testutil.NewFakeClientFactory(),
				test.excludedFromPlans,
			)

			status := apv1beta2.PlanCommandStatus{
				K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{},
			}

			ctx := context.TODO()
			nextState, retry, err := provider.NewPlan(ctx, test.command, &status)

			require.NoError(t, err)
			assert.Equal(t, test.expectedNextState, nextState)
			assert.False(t, retry)
			if assert.NotNil(t, status.AirgapUpdate) {
				assert.True(t, cmp.Equal(test.expectedPlanStatusWorkers, status.AirgapUpdate.Workers, cmpopts.IgnoreFields(apv1beta2.PlanCommandTargetStatus{}, "LastUpdatedTimestamp")))
			}
		})
	}
}
