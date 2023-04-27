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
	"fmt"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"

	"github.com/sirupsen/logrus"
)

type planStateHandler struct {
	logger             *logrus.Entry
	commandProviderMap map[string]PlanCommandProvider
	adapter            PlanStateHandlerAdapter
}

// NewPlanStateHandler creates a new `PlanStateHandler` that will register the supplied `PlanCommandProvider`s,
// and setup delegation to the provided adapter for processing.
func NewPlanStateHandler(logger *logrus.Entry, adapter PlanStateHandlerAdapter, commandProviders ...PlanCommandProvider) PlanStateHandler {
	commandProviderMap := make(map[string]PlanCommandProvider)

	for _, cp := range commandProviders {
		commandProviderMap[cp.CommandID()] = cp
	}

	return &planStateHandler{logger, commandProviderMap, adapter}
}

// Handle will attempt to process the first non-Completed command, delegating its functionality
// to a `PlanCommandProvider`. This provider when completed will determine the status of the
// plan when a failure is involved (retryable vs no-retry). When no error is present, the plan
// will switch to `SchedulableWait` for rescheduling.
//
// When this handler is called with all of its commands marked as `Completed`, it will mark
// the plan status as `Completed`.
func (h *planStateHandler) Handle(ctx context.Context, plan *apv1beta2.Plan) (ProviderResult, error) {
	logger := h.logger.WithField("component", "planstatehandler")

	for cmdIdx, cmd := range plan.Spec.Commands {
		cmdName, cmdHandler, found := planCommandProviderLookup(h.commandProviderMap, cmd)
		if !found {
			// Nothing we can do if we receive a command that we know nothing about.
			return ProviderResultFailure, fmt.Errorf("unknown command state handler '%s'", cmdName)
		}

		// Ensures that we have the same amount of status objects for the number of commands.
		if !ensurePlanStatusSymmetry(plan) {
			return ProviderResultFailure, fmt.Errorf("broken plan status symmetry [#cmd=%d, #status=%d]", len(plan.Spec.Commands), len(plan.Status.Commands))
		}

		// With commands and status having the same index, get the status for the command.
		cmdStatus := findPlanCommandStatus(&plan.Status, cmdIdx)

		// Move to the next command if this one is completed
		if cmdStatus.State == PlanCompleted {
			continue
		}

		// It is the adapters implementation who is responsible for providing the proper status
		// for executing the command.

		originalPlanCommandState := cmdStatus.State
		nextState, retry, err := h.adapter(ctx, cmdHandler, plan.Spec.ID, cmd, cmdStatus)

		// If we're asked to retry, we can ignore any errors and state transition as this is an effective
		// 'redo' of the operation.

		if retry {
			return ProviderResultRetry, nil
		}

		// Explicit errors on the other hand need logging

		if err != nil {
			return ProviderResultFailure, fmt.Errorf("error in plan state adapter: %w", err)
		}

		// If the command has indicated that it is 'Completed', don't use this state for the plan, as its
		// the completion of this loop which determines 'Completed'. This requires another iteration.

		cmdStatus.State = nextState
		if cmdStatus.State != PlanCompleted {
			plan.Status.State = nextState
		}

		if originalPlanCommandState != nextState {
			logger.Infof("Requesting plan command transition from '%s' --> '%s'", originalPlanCommandState, nextState)
		}

		return ProviderResultSuccess, nil
	}

	// If we've completed all of the commands, update our status. The goal is that this can only be reached
	// after successfully identifying that each command is already marked 'Completed'.

	plan.Status.State = PlanCompleted

	return ProviderResultSuccess, nil
}

// ensurePlanStatusSymmetry ensures that if any command status are provided, that they
// match in total with the number of commands in the `Plan`.
func ensurePlanStatusSymmetry(plan *apv1beta2.Plan) bool {
	statusLen := len(plan.Status.Commands)
	if statusLen != 0 {
		return len(plan.Spec.Commands) == statusLen
	}

	return true
}

// findPlanCommandStatus returns the `PlanCommandStatus` at the provided index if available,
// otherwise it adds a new instance to the command status and returns it.
func findPlanCommandStatus(status *apv1beta2.PlanStatus, idx int) *apv1beta2.PlanCommandStatus {
	if idx < len(status.Commands) {
		return &status.Commands[idx]
	}

	// .. otherwise, add a new one and return it
	status.Commands = append(status.Commands, apv1beta2.PlanCommandStatus{ID: idx})

	return &status.Commands[len(status.Commands)-1]
}
