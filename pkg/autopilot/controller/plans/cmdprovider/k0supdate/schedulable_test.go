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

package k0supdate

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/k0sproject/k0s/internal/testutil"
	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	apscheme "github.com/k0sproject/k0s/pkg/client/clientset/scheme"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
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
		expectedPlanStatusControllers []apv1beta2.PlanCommandTargetStatus
		expectedPlanStatusWorkers     []apv1beta2.PlanCommandTargetStatus
		conflictUpdate                bool
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
						Name:   "controller0",
						Labels: map[string]string{corev1.LabelOSStable: "theOS", corev1.LabelArchStable: "theArch"},
					},
				},
			},
			apv1beta2.PlanCommand{
				K0sUpdate: &apv1beta2.PlanCommandK0sUpdate{
					Version: "v99.99.99",
					Platforms: apv1beta2.PlanPlatformResourceURLMap{
						"theOS-theArch": {
							URL:    "https://k0s.example.com/downloads/k0s-v99.99.99-theOS-theArch",
							Sha256: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
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
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("controller0", appc.SignalCompleted),
			},
			nil,
			false,
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
						Labels: map[string]string{corev1.LabelOSStable: "theOS", corev1.LabelArchStable: "theArch"},
					},
				},
			},
			apv1beta2.PlanCommand{
				K0sUpdate: &apv1beta2.PlanCommandK0sUpdate{
					Version: "v99.99.99",
					Platforms: apv1beta2.PlanPlatformResourceURLMap{
						"theOS-theArch": {
							URL:    "https://k0s.example.com/downloads/k0s-v99.99.99-theOS-theArch",
							Sha256: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
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
				ID:    123,
				State: appc.PlanSchedulable,
				K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{
					Controllers: []apv1beta2.PlanCommandTargetStatus{
						apv1beta2.NewPlanCommandTargetStatus("controller0", appc.SignalPending),
					},
				},
			},
			appc.PlanSchedulableWait,
			false,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("controller0", appc.SignalSent),
			},
			nil,
			false,
		},

		// Ensures conflicts on update request a retry and preserve status.
		{
			"ConflictRequeues",
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
			},
			apv1beta2.PlanCommand{
				K0sUpdate: &apv1beta2.PlanCommandK0sUpdate{
					Version: "v99.99.99",
					Platforms: apv1beta2.PlanPlatformResourceURLMap{
						"theOS-theArch": {
							URL:    "https://k0s.example.com/downloads/k0s-v99.99.99-theOS-theArch",
							Sha256: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
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
				ID:    123,
				State: appc.PlanSchedulable,
				K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{
					Controllers: []apv1beta2.PlanCommandTargetStatus{
						apv1beta2.NewPlanCommandTargetStatus("controller0", appc.SignalPending),
					},
				},
			},
			appc.PlanSchedulable,
			true,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("controller0", appc.SignalPending),
			},
			nil,
			true,
		},
	}

	scheme := apimruntime.NewScheme()
	assert.NoError(t, apscheme.AddToScheme(scheme))

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithObjects(test.objects...).WithScheme(scheme)
			if test.conflictUpdate {
				var conflictOnce atomic.Bool
				builder.WithInterceptorFuncs(interceptor.Funcs{
					Update: func(ctx context.Context, client crcli.WithWatch, obj crcli.Object, opts ...crcli.UpdateOption) error {
						if !conflictOnce.Swap(true) {
							return apierrors.NewConflict(schema.GroupResource{
								Group:    "autopilot.k0sproject.io",
								Resource: "controlnodes",
							}, obj.GetName(), errors.New("injected conflict"))
						}
						return client.Update(ctx, obj, opts...)
					},
				})
				t.Cleanup(func() { assert.True(t, conflictOnce.Load(), "Conflict hasn't been injected.") })
			}
			client := builder.Build()

			provider := NewK0sUpdatePlanCommandProvider(
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
			nextState, retry, err := provider.Schedulable(ctx, "id123", test.command, &test.status)

			assert.Equal(t, test.expectedNextState, nextState)
			assert.Equal(t, test.expectedRetry, retry)
			assert.NoError(t, err)

			assert.True(t, cmp.Equal(test.expectedPlanStatusControllers, test.status.K0sUpdate.Controllers, cmpopts.IgnoreFields(apv1beta2.PlanCommandTargetStatus{}, "LastUpdatedTimestamp")))
			assert.True(t, cmp.Equal(test.expectedPlanStatusWorkers, test.status.K0sUpdate.Workers, cmpopts.IgnoreFields(apv1beta2.PlanCommandTargetStatus{}, "LastUpdatedTimestamp")))

		})
	}
}
