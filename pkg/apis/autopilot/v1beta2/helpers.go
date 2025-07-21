// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewPlanCommandTargetStatus creates a new `PlanCommandTargetStatus` with all required defaults populated.
func NewPlanCommandTargetStatus(name string, status PlanCommandTargetStateType) PlanCommandTargetStatus {
	return PlanCommandTargetStatus{
		Name:                 name,
		State:                status,
		LastUpdatedTimestamp: metav1.Now(),
	}
}

// String provides string representation for PlanStateType
func (t PlanStateType) String() string {
	return string(t)
}

// String provides string representation for PlanCommandTargetStateType
func (t PlanCommandTargetStateType) String() string {
	return string(t)
}
