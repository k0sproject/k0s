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

package core

import (
	"context"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
)

// PlanStatusType
var (
	PlanSchedulable     apv1beta2.PlanStateType = "Schedulable"
	PlanSchedulableWait apv1beta2.PlanStateType = "SchedulableWait"
	PlanCompleted       apv1beta2.PlanStateType = "Completed"
	PlanWarning         apv1beta2.PlanStateType = "Warning"

	PlanInconsistentTargets apv1beta2.PlanStateType = "InconsistentTargets"
	PlanIncompleteTargets   apv1beta2.PlanStateType = "IncompleteTargets"
	PlanRestricted          apv1beta2.PlanStateType = "Restricted"
	PlanMissingSignalNode   apv1beta2.PlanStateType = "MissingSignalNode"
	PlanApplyFailed         apv1beta2.PlanStateType = "ApplyFailed"
)

// PlanCommandStatusType
var (
	SignalPending         apv1beta2.PlanCommandTargetStateType = "SignalPending"
	SignalSent            apv1beta2.PlanCommandTargetStateType = "SignalSent"
	SignalCompleted       apv1beta2.PlanCommandTargetStateType = "SignalCompleted"
	SignalMissingNode     apv1beta2.PlanCommandTargetStateType = "SignalMissingNode"
	SignalMissingPlatform apv1beta2.PlanCommandTargetStateType = "SignalMissingPlatform"
	SignalApplyFailed     apv1beta2.PlanCommandTargetStateType = "SignalApplyFailed"
)

type ProviderResult int

const (
	ProviderResultSuccess ProviderResult = iota
	ProviderResultFailure
	ProviderResultRetry
)

// PlanStateHandler defines the implementation of how a `PlanStateController` will perform
// its reconciliation.
type PlanStateHandler interface {
	// Handle handles the reconciliation against the provided `Plan`, returning the `PlanStatus`
	// if there is a change in status, or nil if no change is required.
	Handle(ctx context.Context, plan *apv1beta2.Plan) (ProviderResult, error)
}

// PlanStateHandlerAdapter defines an adapter function between the `PlanStateController`, and the
// specific function to call in the resolved `PlanCommandProvider`.
type PlanStateHandlerAdapter func(ctx context.Context, provider PlanCommandProvider, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error)

// PlanCommandProviderMap is a mapping of command names to `PlanCommandProvider` instances.
type PlanCommandProviderMap map[string]PlanCommandProvider

// PlanCommandProvider defines what a specific `PlanCommand` can do at specific states.
//
// The methods provided represent the various states that a `Plan` can transition to, and their
// implementations provide the behavior of the command when reaching this state.
type PlanCommandProvider interface {
	// CommandID is the identifier of the command which needs to match the field name of the
	// command in `PlanCommand`.
	CommandID() string

	// NewPlan handles the provider state 'newplan'
	NewPlan(ctx context.Context, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error)

	// Schedulable handles the provider state 'schedulable'
	Schedulable(ctx context.Context, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error)

	// SchedulableWait handles the provider state 'schedulablewait'
	SchedulableWait(ctx context.Context, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error)
}
