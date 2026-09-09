// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
	"k8s.io/apimachinery/pkg/types"

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

		// Ensures that a stale error annotation on a ControlNode is cleared before the new signal is sent.
		{
			"ControlNodeErrorClearedOnNewSignal",
			[]crcli.Object{
				&apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:   "controller0",
						Labels: map[string]string{corev1.LabelOSStable: "theOS", corev1.LabelArchStable: "theArch"},
						Annotations: map[string]string{
							apdel.SignalErrorAnnotation: `{"planID":"oldplan","reason":"FailedDownload","message":"stale error","timestamp":"2024-01-01T00:00:00Z"}`,
						},
					},
				},
			},
			apv1beta2.PlanCommand{
				K0sUpdate: &apv1beta2.PlanCommandK0sUpdate{
					Version: "v99.99.99",
					Platforms: apv1beta2.PlanPlatformResourceURLMap{
						"theOS-theArch": {URL: "https://k0s.example.com/downloads/k0s-v99.99.99-theOS-theArch"},
					},
					Targets: apv1beta2.PlanCommandTargets{
						Controllers: apv1beta2.PlanCommandTarget{
							Discovery: apv1beta2.PlanCommandTargetDiscovery{
								Static: &apv1beta2.PlanCommandTargetDiscoveryStatic{Nodes: []string{"controller0"}},
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

		// Ensures that a stale last-error annotation on a Node is cleared before the new signal is sent.
		{
			"NodeErrorAnnotationClearedOnNewSignal",
			[]crcli.Object{
				&corev1.Node{
					TypeMeta: metav1.TypeMeta{Kind: "Node", APIVersion: "v1"},
					ObjectMeta: metav1.ObjectMeta{
						Name:   "worker0",
						Labels: map[string]string{corev1.LabelOSStable: "theOS", corev1.LabelArchStable: "theArch"},
						Annotations: map[string]string{
							"k0sproject.io/autopilot-last-error": `{"planID":"oldplan","reason":"FailedDownload","message":"stale error","timestamp":"2024-01-01T00:00:00Z"}`,
						},
					},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
					},
				},
			},
			apv1beta2.PlanCommand{
				K0sUpdate: &apv1beta2.PlanCommandK0sUpdate{
					Version: "v99.99.99",
					Platforms: apv1beta2.PlanPlatformResourceURLMap{
						"theOS-theArch": {URL: "https://k0s.example.com/downloads/k0s-v99.99.99-theOS-theArch"},
					},
					Targets: apv1beta2.PlanCommandTargets{
						Workers: apv1beta2.PlanCommandTarget{
							Discovery: apv1beta2.PlanCommandTargetDiscovery{
								Static: &apv1beta2.PlanCommandTargetDiscoveryStatic{Nodes: []string{"worker0"}},
							},
							Limits: apv1beta2.PlanCommandTargetLimits{Concurrent: 1},
						},
					},
				},
			},
			apv1beta2.PlanCommandStatus{
				ID:    123,
				State: appc.PlanSchedulable,
				K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{
					Workers: []apv1beta2.PlanCommandTargetStatus{
						apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalPending),
					},
				},
			},
			appc.PlanSchedulableWait,
			false,
			nil,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalSent),
			},
			false,
		},
	}

	scheme := apimruntime.NewScheme()
	assert.NoError(t, apscheme.AddToScheme(scheme))
	assert.NoError(t, corev1.AddToScheme(scheme))

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

			if test.name == "ControlNodeErrorClearedOnNewSignal" {
				cn := &apv1beta2.ControlNode{}
				assert.NoError(t, client.Get(ctx, types.NamespacedName{Name: "controller0"}, cn))
				_, hasAnnotation := cn.GetAnnotations()[apdel.SignalErrorAnnotation]
				assert.False(t, hasAnnotation, "error annotation should be cleared before new signal")
			}
			if test.name == "NodeErrorAnnotationClearedOnNewSignal" {
				node := &corev1.Node{}
				assert.NoError(t, client.Get(ctx, types.NamespacedName{Name: "worker0"}, node))
				_, hasAnnotation := node.GetAnnotations()[apdel.SignalErrorAnnotation]
				assert.False(t, hasAnnotation)
			}

		})
	}
}
