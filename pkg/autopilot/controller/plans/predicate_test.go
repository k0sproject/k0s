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
	assert.Equal(t, true, pred.Create(crev.CreateEvent{Object: createPlan("foo")}))
	assert.Equal(t, true, pred.Update(crev.UpdateEvent{ObjectNew: createPlan("foo")}))
	assert.Equal(t, false, pred.Create(crev.CreateEvent{Object: createPlan("bar")}))
	assert.Equal(t, false, pred.Update(crev.UpdateEvent{ObjectNew: createPlan("bar")}))
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
	assert.Equal(t, true, pred.Update(crev.UpdateEvent{ObjectNew: createPlan(appc.PlanSchedulable)}))
	assert.Equal(t, true, pred.Create(crev.CreateEvent{Object: createPlan(appc.PlanSchedulable)}))
	assert.Equal(t, false, pred.Update(crev.UpdateEvent{ObjectNew: createPlan("unknown")}))
	assert.Equal(t, false, pred.Create(crev.CreateEvent{Object: createPlan("unknown")}))
}
