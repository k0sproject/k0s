// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
