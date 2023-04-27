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
	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"

	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crpred "sigs.k8s.io/controller-runtime/pkg/predicate"
)

// PlanStatusPredicate creates a controller-runtime predicate that ensures that the object
// in question is a `plan` that has the provided status.
func PlanStatusPredicate(status apv1beta2.PlanStateType) crpred.Predicate {
	return crpred.NewPredicateFuncs(func(obj crcli.Object) bool {
		plan, ok := obj.(*apv1beta2.Plan)
		return ok && plan.Status.State == status
	})
}

// PlanNamePredicate creates a controller-runtime predicate that ensures that the
// object in question has the provided name.
func PlanNamePredicate(name string) crpred.Predicate {
	return crpred.NewPredicateFuncs(func(obj crcli.Object) bool {
		plan, ok := obj.(*apv1beta2.Plan)
		return ok && plan.Name == name
	})
}
