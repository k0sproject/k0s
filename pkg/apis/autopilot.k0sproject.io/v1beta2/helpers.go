// Copyright 2022 k0s authors
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
