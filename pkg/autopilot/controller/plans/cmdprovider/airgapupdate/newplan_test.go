// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package airgapupdate

import (
	"testing"

	"github.com/k0sproject/k0s/internal/testutil"
	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	apscheme "github.com/k0sproject/k0s/pkg/client/clientset/scheme"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
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
				&corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:   "worker0",
						Labels: map[string]string{corev1.LabelOSStable: "theOS", corev1.LabelArchStable: "theArch"},
					},
				},
			},
			apv1beta2.PlanCommand{
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdate{
					Platforms: apv1beta2.PlanPlatformResourceURLMap{
						"theOS-theArch": {},
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
				&corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:   "workerMISSING",
						Labels: map[string]string{corev1.LabelOSStable: "theOS", corev1.LabelArchStable: "theArch"},
					},
				},
			},
			apv1beta2.PlanCommand{
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdate{
					Platforms: apv1beta2.PlanPlatformResourceURLMap{
						"theOS-theArch": {},
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
						Labels: map[string]string{corev1.LabelOSStable: "theOS", corev1.LabelArchStable: "theArch"},
					},
				},
				&corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:   "worker0",
						Labels: map[string]string{corev1.LabelOSStable: "anotherOS", corev1.LabelArchStable: "theArch"},
					},
				},
			},
			apv1beta2.PlanCommand{
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdate{
					Platforms: apv1beta2.PlanPlatformResourceURLMap{
						"theOS-theArch": {},
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
				&corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:   "worker0",
						Labels: map[string]string{corev1.LabelOSStable: "theOS", corev1.LabelArchStable: "theArch"},
					},
				},
			},
			apv1beta2.PlanCommand{
				AirgapUpdate: &apv1beta2.PlanCommandAirgapUpdate{
					Platforms: apv1beta2.PlanPlatformResourceURLMap{
						"theOS-theArch": {},
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
	assert.NoError(t, corev1.AddToScheme(scheme))

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

			ctx := t.Context()
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
