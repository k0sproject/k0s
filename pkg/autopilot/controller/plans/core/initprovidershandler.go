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

package core

import (
	"context"
	"fmt"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"

	"github.com/sirupsen/logrus"
)

type initProvidersHandler struct {
	logger               *logrus.Entry
	commandProviderMap   PlanCommandProviderMap
	adapter              PlanStateHandlerAdapter
	expectedSuccessState apv1beta2.PlanStateType
}

var _ PlanStateHandler = (*initProvidersHandler)(nil)

// NewInitProvidersHandler creates a new `PlanStateHandler` that will register the supplied `PlanCommandProvider`s,
// and setup delegation to the provided adapter for processing.
func NewInitProvidersHandler(logger *logrus.Entry, adapter PlanStateHandlerAdapter, successState apv1beta2.PlanStateType, commandProviders ...PlanCommandProvider) PlanStateHandler {
	commandProviderMap := make(map[string]PlanCommandProvider)

	for _, cp := range commandProviders {
		commandProviderMap[cp.CommandID()] = cp
	}

	return &initProvidersHandler{
		logger:               logger,
		commandProviderMap:   commandProviderMap,
		adapter:              adapter,
		expectedSuccessState: successState,
	}
}

// Handle will iterate across all of the commands in the plan, creates a matching `PlanCommandStatus`,
// and then delegates to the configured adapter for command-specific processing. This gives all of the
// providers the reassurance that a status will be available for them to populate.
func (ah *initProvidersHandler) Handle(ctx context.Context, plan *apv1beta2.Plan) (ProviderResult, error) {
	logger := ah.logger.WithField("component", "inithandler")

	for cmdIdx, cmd := range plan.Spec.Commands {
		cmdName, cmdHandler, found := planCommandProviderLookup(ah.commandProviderMap, cmd)
		if !found {
			// Nothing we can do if we receive a command that we know nothing about.
			return ProviderResultFailure, fmt.Errorf("unknown command state handler '%s'", cmdName)
		}

		// Create an empty status for the command.

		logger.Infof("Adding new status for plan '%s' (index=%d)", cmdName, cmdIdx)
		plan.Status.Commands = append(plan.Status.Commands, apv1beta2.PlanCommandStatus{ID: cmdIdx})

		// It is the adapters implementation who is responsible for providing the proper status
		// for executing the command.

		nextState, _, err := ah.adapter(ctx, cmdHandler, plan.Spec.ID, cmd, &plan.Status.Commands[len(plan.Status.Commands)-1])

		// Given that this is a fixed-initialization, we expect that all of the command initialization should
		// succeed, but in the case that it doesn't make the caller aware.

		if nextState != ah.expectedSuccessState {
			logger.Infof("Unsuccessful state '%s' (expected '%s') from '%s' handler, ending", nextState, ah.expectedSuccessState, cmdName)
			plan.Status.State = nextState
			return ProviderResultSuccess, nil
		}

		// Explicit errors on the other hand need logging

		if err != nil {
			return ProviderResultFailure, fmt.Errorf("error in all-providers handler: %w", err)
		}
	}

	plan.Status.State = PlanSchedulableWait

	return ProviderResultSuccess, nil
}
