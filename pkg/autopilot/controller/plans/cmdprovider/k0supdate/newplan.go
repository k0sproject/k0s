// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package k0supdate

import (
	"context"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	"github.com/k0sproject/k0s/pkg/autopilot/checks"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	appkd "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/cmdprovider/k0supdate/discovery"
	appku "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/cmdprovider/k0supdate/utils"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

// NewPlan handles the provider state 'newplan'
func (kp *k0supdate) NewPlan(ctx context.Context, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
	logger := kp.logger.WithField("state", "newplan")
	logger.Info("Processing")

	if !cmd.K0sUpdate.ForceUpdate {
		if err := checks.CanUpdate(ctx, logger, kp.cf, cmd.K0sUpdate.Version); err != nil {
			status.State = appc.PlanWarning
			status.Description = err.Error()
			return appc.PlanWarning, false, err
		}
	}

	// Setup the response status
	status.State = appc.PlanSchedulableWait
	status.K0sUpdate = &apv1beta2.PlanCommandK0sUpdateStatus{}

	var allControllersAccountedFor bool
	status.K0sUpdate.Controllers, allControllersAccountedFor = populateControllerStatus(ctx, kp.client, *cmd.K0sUpdate, kp.controllerDelegateMap)
	if !allControllersAccountedFor {
		kp.logger.Warnf("Not all controllers accounted for: %v", status.K0sUpdate.Controllers)
	}

	var allWorkersAccountedFor bool
	status.K0sUpdate.Workers, allWorkersAccountedFor = populateWorkerStatus(ctx, kp.client, *cmd.K0sUpdate, kp.controllerDelegateMap)
	if !allWorkersAccountedFor {
		kp.logger.Warnf("Not all workers accounted for: %v", status.K0sUpdate.Workers)
	}

	if !allControllersAccountedFor || !allWorkersAccountedFor {
		return appc.PlanIncompleteTargets, false, nil
	}

	// With the work done for this command, determine if the content should be restricted. Performing this
	// assertion after processing prevents keeps this function consistent in that the content is guaranteed
	// to be processed (vs. exiting early with incomplete results)

	if _, found := kp.excludedFromPlans["controller"]; found && len(status.K0sUpdate.Controllers) > 0 {
		return appc.PlanRestricted, false, nil
	}

	if _, found := kp.excludedFromPlans["worker"]; found && len(status.K0sUpdate.Workers) > 0 {
		return appc.PlanRestricted, false, nil
	}

	return appc.PlanSchedulableWait, false, nil
}

// populateControllerStatus is a specialization of `DiscoverNodes` for working
// with `apv1beta2.ControlNode` signal node objects.
func populateControllerStatus(ctx context.Context, client crcli.Client, update apv1beta2.PlanCommandK0sUpdate, dm apdel.ControllerDelegateMap) ([]apv1beta2.PlanCommandTargetStatus, bool) {
	return appkd.DiscoverNodes(ctx, client, &update.Targets.Controllers, dm["controller"],
		func(name string) (appkd.SignalObjectFilterResult, *apv1beta2.PlanCommandTargetStateType) {
			if exists, state := appku.ObjectExistsWithPlatform(ctx, client, name, &apv1beta2.ControlNode{}, update.Platforms); exists {
				return appkd.SignalObjectFilterResultFound, state
			} else {
				return appkd.SignalObjectFilterResultMissing, state
			}
		})
}

// populateWorkerStatus is a specialization of `DiscoverNodes` for working
// with `v1.Node` signal node objects.
func populateWorkerStatus(ctx context.Context, client crcli.Client, update apv1beta2.PlanCommandK0sUpdate, dm apdel.ControllerDelegateMap) ([]apv1beta2.PlanCommandTargetStatus, bool) {
	worker := dm["worker"]
	return appkd.DiscoverNodes(ctx, client, &update.Targets.Workers, worker, func(name string) (appkd.SignalObjectFilterResult, *apv1beta2.PlanCommandTargetStateType) {
		exists, state := appku.ObjectExistsWithPlatform(ctx, client, name, worker.CreateObject(), update.Platforms)
		if !exists {
			return appkd.SignalObjectFilterResultMissing, state
		}

		// Ensure this is a pure worker, i.e. there's no corresponding
		// controller object. Signals for controllers with embedded workers are
		// purely handled via their controller objects.
		controller := dm["controller"]
		exists, _ = appku.ObjectExistsWithPlatform(ctx, client, name, controller.CreateObject(), update.Platforms)
		if exists {
			return appkd.SignalObjectFilterResultIgnore, state
		}

		return appkd.SignalObjectFilterResultFound, state
	})
}
