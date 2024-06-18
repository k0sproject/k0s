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

package airgapupdate

import (
	"context"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	"github.com/k0sproject/k0s/pkg/autopilot/checks"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
)

// NewPlan handles the provider state 'newplan'
func (aup *airgapupdate) NewPlan(ctx context.Context, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
	logger := aup.logger.WithField("state", "newplan")
	logger.Info("Processing")

	if err := checks.CanUpdate(ctx, logger, aup.cf, cmd.AirgapUpdate.Version); err != nil {
		status.State = appc.PlanWarning
		status.Description = err.Error()
		return appc.PlanWarning, false, err
	}

	// Setup the response status
	status.State = appc.PlanSchedulableWait
	status.AirgapUpdate = &apv1beta2.PlanCommandAirgapUpdateStatus{}

	var allWorkersAccountedFor bool
	status.AirgapUpdate.Workers, allWorkersAccountedFor = populateWorkerStatus(ctx, aup.client, *cmd.AirgapUpdate, aup.controllerDelegateMap)

	if !allWorkersAccountedFor {
		return appc.PlanIncompleteTargets, false, nil
	}

	if _, found := aup.excludedFromPlans["worker"]; found && len(status.AirgapUpdate.Workers) > 0 {
		return appc.PlanRestricted, false, nil
	}

	return appc.PlanSchedulableWait, false, nil
}
