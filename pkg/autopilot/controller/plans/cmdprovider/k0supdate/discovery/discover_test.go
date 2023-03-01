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

package discovery

import (
	"context"
	"testing"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	apscheme "github.com/k0sproject/k0s/pkg/client/clientset/scheme"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestDiscover covers the scenarios of turning target objects into matching status objects,
// with proper status.
func TestDiscover(t *testing.T) {
	alwaysExists := func(name string) (bool, *apv1beta2.PlanCommandTargetStateType) {
		return true, nil
	}

	alwaysExistsExceptFor := func(exception string) SignalObjectExistsFunc {
		return func(name string) (bool, *apv1beta2.PlanCommandTargetStateType) {
			if name == exception {
				return false, &appc.SignalMissingNode
			}

			return true, nil
		}
	}

	var tests = []struct {
		name                    string
		target                  apv1beta2.PlanCommandTarget
		delegate                apdel.ControllerDelegate
		objects                 []crcli.Object
		exists                  SignalObjectExistsFunc
		expectedStatusNodes     []apv1beta2.PlanCommandTargetStatus
		expectedAllAccountedFor bool
	}{
		{
			"StaticControllerAndWorker",
			apv1beta2.PlanCommandTarget{
				Discovery: apv1beta2.PlanCommandTargetDiscovery{
					Static: &apv1beta2.PlanCommandTargetDiscoveryStatic{
						Nodes: []string{"controller0", "worker0"},
					},
				},
			},
			apdel.ControlNodeControllerDelegate(),
			[]crcli.Object{},
			alwaysExists,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("controller0", appc.SignalPending),
				apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalPending),
			},
			true,
		},
		{
			"StaticMissingWorker",
			apv1beta2.PlanCommandTarget{
				Discovery: apv1beta2.PlanCommandTargetDiscovery{
					Static: &apv1beta2.PlanCommandTargetDiscoveryStatic{
						Nodes: []string{"controller0", "worker0"},
					},
				},
			},
			apdel.ControlNodeControllerDelegate(),
			[]crcli.Object{},
			alwaysExistsExceptFor("worker0"),
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("controller0", appc.SignalPending),
				apv1beta2.NewPlanCommandTargetStatus("worker0", appc.SignalMissingNode),
			},
			false,
		},
		{
			"SelectorLabelNoMatch",
			apv1beta2.PlanCommandTarget{
				Discovery: apv1beta2.PlanCommandTargetDiscovery{
					Selector: &apv1beta2.PlanCommandTargetDiscoverySelector{
						Labels: "foo=bar",
					},
				},
			},
			apdel.ControlNodeControllerDelegate(),
			[]crcli.Object{
				&apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "controller0",
					},
				},
			},
			alwaysExists,
			[]apv1beta2.PlanCommandTargetStatus{},
			true,
		},
		{
			"SelectorLabelExcludeOne",
			apv1beta2.PlanCommandTarget{
				Discovery: apv1beta2.PlanCommandTargetDiscovery{
					Selector: &apv1beta2.PlanCommandTargetDiscoverySelector{
						Labels: "foo=bar",
					},
				},
			},
			apdel.ControlNodeControllerDelegate(),
			[]crcli.Object{
				&apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "controller0",
						Labels: map[string]string{
							"foo": "bar",
						},
					},
				},
				&apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "controller1",
					},
				},
			},
			alwaysExists,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("controller0", appc.SignalPending),
			},
			true,
		},
		{
			"SelectorAll",
			apv1beta2.PlanCommandTarget{
				Discovery: apv1beta2.PlanCommandTargetDiscovery{
					Selector: &apv1beta2.PlanCommandTargetDiscoverySelector{},
				},
			},
			apdel.ControlNodeControllerDelegate(),
			[]crcli.Object{
				&apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "controller0",
						Labels: map[string]string{
							"foo": "bar",
						},
					},
				},
				&apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "controller1",
					},
				},
			},
			alwaysExists,
			[]apv1beta2.PlanCommandTargetStatus{
				apv1beta2.NewPlanCommandTargetStatus("controller0", appc.SignalPending),
				apv1beta2.NewPlanCommandTargetStatus("controller1", appc.SignalPending),
			},
			true,
		},

		// TODO: Can't use field selectors with the controller-runtime fake client (not implemented),
		//       so we need to rely on integration tests for this.
		//
		// https://github.com/kubernetes-sigs/controller-runtime/issues/1376
	}

	scheme := runtime.NewScheme()
	assert.NoError(t, apscheme.AddToScheme(scheme))
	assert.NoError(t, v1.AddToScheme(scheme))

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := crfake.NewClientBuilder().WithObjects(test.objects...).WithScheme(scheme).Build()

			statusNodes, allAccountedFor := DiscoverNodes(context.TODO(), client, &test.target, test.delegate, test.exists)
			assert.True(t, cmp.Equal(test.expectedStatusNodes, statusNodes, cmpopts.IgnoreFields(apv1beta2.PlanCommandTargetStatus{}, "LastUpdatedTimestamp")))
			assert.Equal(t, test.expectedAllAccountedFor, allAccountedFor)
		})
	}
}
