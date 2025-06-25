// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
