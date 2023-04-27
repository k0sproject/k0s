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

package plans

import (
	"fmt"
	"testing"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crev "sigs.k8s.io/controller-runtime/pkg/event"
)

// TestNewPlanEventFilter ensures that only create events make it through
// the predicate evaluation.
func TestNewPlanEventFilter(t *testing.T) {
	pred := newPlanEventFilter()

	plan := &apv1beta2.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name: apconst.AutopilotName,
		},
	}

	assert.True(t, pred.Create(crev.CreateEvent{Object: plan}))
	assert.False(t, pred.Delete(crev.DeleteEvent{Object: plan}))
	assert.False(t, pred.Generic(crev.GenericEvent{Object: plan}))
	assert.False(t, pred.Update(crev.UpdateEvent{ObjectNew: plan}))

	// Test with an incorrect name
	planBad := &apv1beta2.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}

	assert.False(t, pred.Create(crev.CreateEvent{Object: planBad}))
	assert.False(t, pred.Delete(crev.DeleteEvent{Object: planBad}))
	assert.False(t, pred.Generic(crev.GenericEvent{Object: planBad}))
	assert.False(t, pred.Update(crev.UpdateEvent{ObjectNew: planBad}))
}

// TestNewPlanEventFilter_withStatus ensures that only create events make it through
// the predicate evaluation that have an empty status.
func TestNewPlanEventFilter_withStatus(t *testing.T) {
	pred := newPlanEventFilter()

	plan := &apv1beta2.Plan{
		Status: apv1beta2.PlanStatus{
			State: "something",
		},
	}

	assert.False(t, pred.Create(crev.CreateEvent{Object: plan}))
	assert.False(t, pred.Delete(crev.DeleteEvent{Object: plan}))
	assert.False(t, pred.Generic(crev.GenericEvent{Object: plan}))
	assert.False(t, pred.Update(crev.UpdateEvent{ObjectNew: plan}))
}

// TestSchedulableWaitEventFilter ensures that only certain events make it through
// the predicate evaluation.
func TestSchedulableWaitEventFilter(t *testing.T) {
	var tests = []struct {
		name     string
		plan     *apv1beta2.Plan
		expected bool
	}{
		{
			"Happy",
			&apv1beta2.Plan{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Plan",
					APIVersion: "autopilot.k0sproject.io/v1beta2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: apconst.AutopilotName,
				},
				Status: apv1beta2.PlanStatus{
					State: appc.PlanSchedulableWait,
				},
			},
			true,
		},
		{
			"WrongStatus",
			&apv1beta2.Plan{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Plan",
					APIVersion: "autopilot.k0sproject.io/v1beta2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: apconst.AutopilotName,
				},
				Status: apv1beta2.PlanStatus{
					State: appc.PlanCompleted,
				},
			},
			false,
		},
		{
			"WrongName",
			&apv1beta2.Plan{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Plan",
					APIVersion: "autopilot.k0sproject.io/v1beta2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "Foo",
				},
				Status: apv1beta2.PlanStatus{
					State: appc.PlanCompleted,
				},
			},
			false,
		},
	}

	pred := schedulableWaitEventFilter()

	for idx, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := crev.UpdateEvent{ObjectNew: test.plan}
			assert.Equal(t, test.expected, pred.Update(event), fmt.Sprintf("Failed at #%d '%s'", idx, test.name))
		})
	}
}

// TestSchedulableEventFilter ensures that only create events make it through
// the predicate evaluation.
func TestSchedulableEventFilter(t *testing.T) {
	var tests = []struct {
		name     string
		plan     *apv1beta2.Plan
		expected bool
	}{
		{
			"Happy",
			&apv1beta2.Plan{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Plan",
					APIVersion: "autopilot.k0sproject.io/v1beta2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: apconst.AutopilotName,
				},
				Status: apv1beta2.PlanStatus{
					State: appc.PlanSchedulable,
				},
			},
			true,
		},
		{
			"Wrong status",
			&apv1beta2.Plan{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Plan",
					APIVersion: "autopilot.k0sproject.io/v1beta2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: apconst.AutopilotName,
				},
				Status: apv1beta2.PlanStatus{
					State: appc.PlanCompleted,
				},
			},
			false,
		},
		{
			"Wrong name",
			&apv1beta2.Plan{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Plan",
					APIVersion: "autopilot.k0sproject.io/v1beta2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "Foo",
				},
				Status: apv1beta2.PlanStatus{
					State: appc.PlanCompleted,
				},
			},
			false,
		},
	}

	pred := schedulableEventFilter()

	for idx, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := crev.UpdateEvent{ObjectNew: test.plan}
			assert.Equal(t, test.expected, pred.Update(event), fmt.Sprintf("Failed at #%d '%s'", idx, test.name))
		})
	}
}
