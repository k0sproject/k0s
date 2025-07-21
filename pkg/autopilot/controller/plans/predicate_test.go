// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package plans

import (
	"testing"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crev "sigs.k8s.io/controller-runtime/pkg/event"
)

// TestPlanNamePredicate ensures that plans can be identified by their name
func TestPlanNamePredicate(t *testing.T) {
	createPlan := func(name string) *apv1beta2.Plan {
		return &apv1beta2.Plan{
			ObjectMeta: v1.ObjectMeta{
				Name: name,
			},
		}
	}

	pred := PlanNamePredicate("foo")
	assert.True(t, pred.Create(crev.CreateEvent{Object: createPlan("foo")}))
	assert.True(t, pred.Update(crev.UpdateEvent{ObjectNew: createPlan("foo")}))
	assert.False(t, pred.Create(crev.CreateEvent{Object: createPlan("bar")}))
	assert.False(t, pred.Update(crev.UpdateEvent{ObjectNew: createPlan("bar")}))
}

// TestPlanStatusPredicate ensures that plans can be identified by their status
// across multiple states.
func TestPlanStatusPredicate(t *testing.T) {
	createPlan := func(status apv1beta2.PlanStateType) *apv1beta2.Plan {
		return &apv1beta2.Plan{
			ObjectMeta: v1.ObjectMeta{
				Name: "autopilot",
			},
			Status: apv1beta2.PlanStatus{
				State: status,
			},
		}
	}

	pred := PlanStatusPredicate(appc.PlanSchedulable)
	assert.True(t, pred.Update(crev.UpdateEvent{ObjectNew: createPlan(appc.PlanSchedulable)}))
	assert.True(t, pred.Create(crev.CreateEvent{Object: createPlan(appc.PlanSchedulable)}))
	assert.False(t, pred.Update(crev.UpdateEvent{ObjectNew: createPlan("unknown")}))
	assert.False(t, pred.Create(crev.CreateEvent{Object: createPlan("unknown")}))
}
