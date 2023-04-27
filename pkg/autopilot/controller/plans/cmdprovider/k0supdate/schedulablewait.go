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

package k0supdate

import (
	"context"
	"fmt"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	appku "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/cmdprovider/k0supdate/utils"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	apsigcomm "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"
)

// SchedulableWait handles the provider state 'schedulablewait'
func (kp *k0supdate) SchedulableWait(ctx context.Context, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
	logger := kp.logger.WithField("state", "schedulablewait")
	logger.Info("Processing")

	// Update the target status for both controllers and workers based on queries
	// of their respective signal node objects.

	logger.Info("Reconciling controller/worker signal node statuses")
	if err := kp.reconcileSignalNodeStatus(ctx, planID, status); err != nil {
		return status.State, false, fmt.Errorf("failed to reconcile signal node status: %w", err)
	}

	// If any of the nodes have reported a failure in applying an update, the
	// plan is marked as a failure.

	if appku.IsNotRecoverable(status.K0sUpdate.Controllers, status.K0sUpdate.Workers) {
		logger.Info("Plan is non-recoverable due to apply failure")
		return appc.PlanApplyFailed, false, nil
	}

	controllersDone := appku.IsCompleted(status.K0sUpdate.Controllers)
	workersDone := appku.IsCompleted(status.K0sUpdate.Workers)

	if controllersDone && workersDone {
		logger.Info("Controllers and workers completed")
		return appc.PlanCompleted, false, nil
	}

	canScheduleController, _ := isSchedulableControllers(status.K0sUpdate.Controllers)
	canScheduleWorkers, _ := isSchedulableWorkers(cmd.K0sUpdate.Targets.Workers, status.K0sUpdate.Workers)

	// Controllers have priority for scheduling evaluation, as it is important that controllers
	// are updated before workers due to the Kubernetes version-skew policy.
	//
	// https://kubernetes.io/releases/version-skew-policy/

	if !controllersDone && canScheduleController {
		logger.Info("Controllers can be scheduled")
		return appc.PlanSchedulable, false, nil
	}

	// Only once controllers are done can we consider workers.

	if !workersDone && canScheduleWorkers && controllersDone {
		logger.Info("Workers can be scheduled (controllers done)")
		return appc.PlanSchedulable, false, nil
	}

	logger.Info("No applicable transitions available, requesting retry")
	return appc.PlanSchedulableWait, true, nil
}

// reconcileSignalNodeStatus performs a reconciliation of the status of every signal node (controller/worker)
// defined in the update status, ensuring that signal nodes marked as 'Completed' are updated in the plan status.
func (kp *k0supdate) reconcileSignalNodeStatus(ctx context.Context, planID string, cmdStatus *apv1beta2.PlanCommandStatus) error {
	var targets = []struct {
		nodes []apv1beta2.PlanCommandTargetStatus
		label string
	}{
		{cmdStatus.K0sUpdate.Controllers, "controller"},
		{cmdStatus.K0sUpdate.Workers, "worker"},
	}

	for _, target := range targets {
		delegate, found := kp.controllerDelegateMap[target.label]
		if !found {
			return fmt.Errorf("unable to find controller delegate '%s'", target.label)
		}

		kp.reconcileSignalNodeStatusTarget(ctx, planID, *cmdStatus, delegate, target.nodes)
	}

	return nil
}

// reconcileSignalNodeStatusTarget performs a reconciliation of the status of every signal node provided
// against the current state maintained in the plan status. This ensures that any signal nodes that
// have been transitioned to 'Completed' will also appear in the plan status as 'Completed'.
func (kp *k0supdate) reconcileSignalNodeStatusTarget(ctx context.Context, planID string, cmdStatus apv1beta2.PlanCommandStatus, delegate apdel.ControllerDelegate, signalNodes []apv1beta2.PlanCommandTargetStatus) {
	for i := 0; i < len(signalNodes); i++ {
		key := delegate.CreateNamespacedName(signalNodes[i].Name)
		signalNode := delegate.CreateObject()

		if err := kp.client.Get(ctx, key, signalNode); err != nil {
			kp.logger.Warnf("Unable to find signal node '%s'", signalNodes[i].Name)
			continue
		}

		if apsigv2.IsSignalingPresent(signalNode.GetAnnotations()) {
			var signalData apsigv2.SignalData
			if err := signalData.Unmarshal(signalNode.GetAnnotations()); err == nil {
				if signalData.PlanID == planID {
					if signalNodes[i].State == appc.SignalCompleted {
						continue
					}

					// Ensure that the commands are the same, but their status's are different before we check completed.
					if appku.IsSignalDataSameCommand(cmdStatus, signalData) && appku.IsSignalDataStatusDifferent(signalNodes[i], signalData.Status) {
						origState := signalNodes[i].State

						if signalData.Status.Status == apsigcomm.Failed || signalData.Status.Status == apsigcomm.FailedDownload {
							signalNodes[i].State = appc.SignalApplyFailed
						}

						if signalData.Status.Status == apsigcomm.Completed {
							signalNodes[i].State = appc.SignalCompleted
						}

						kp.logger.Infof("Signal node '%s' status changed from '%s' to '%s' (reason: %s)", signalNodes[i].Name, origState, signalNodes[i].State, signalData.Status.Status)
					}
				} else {
					kp.logger.Warnf("Current planid '%v' doesn't match signal node planid '%v'", planID, signalData.PlanID)
				}
			} else {
				kp.logger.Warnf("Unable to unmarshal signaling data from signal node '%s'", signalNode.GetName())
			}
		}
	}
}

// isSchedulableControllers determines if any of the controllers in the plan status have
// a status which would a) require the plan to become `schedulable`, or b) have enough
// information to exclude controllers from the determination.
func isSchedulableControllers(status []apv1beta2.PlanCommandTargetStatus) (canSchedule bool, exclude bool) {
	return isSchedulable(status, func(pendingSignalCount, signalingSentCount int) bool {
		return signalingSentCount == 0
	})
}

// isSchedulableWorkers determines if any of the workers in the plan status have
// a status which would require the plan to become `schedulable`.
func isSchedulableWorkers(target apv1beta2.PlanCommandTarget, workers []apv1beta2.PlanCommandTargetStatus) (canSchedule, exclude bool) {
	return isSchedulable(workers, func(pendingSignalCount, signalingSentCount int) bool {
		return signalingSentCount < target.Limits.Concurrent
	})
}

type targetScheduleCondition func(pendingSignalCount, signalingSentCount int) bool

// isSchedulable determines if the provided collection of PlanCommandTargetStatus can be considered
// as scehdulable. The predicate evaluation delegates to an external schedule condition for specialization.
func isSchedulable(status []apv1beta2.PlanCommandTargetStatus, cond targetScheduleCondition) (canSchedule, exclude bool) {
	pendingSignalCount, signalingSentCount := countPlanCommandTargetStatus(status)

	canSchedule = pendingSignalCount > 0 && cond(pendingSignalCount, signalingSentCount)
	exclude = pendingSignalCount == 0 && signalingSentCount == 0

	return
}

// countPlanCommandTargetStatus iterates over the provided slice of PlanCommandTargetStatus,
// returning a count of nodes in PendingSignal and SignalingSent.
func countPlanCommandTargetStatus(nodes []apv1beta2.PlanCommandTargetStatus) (pendingSignalCount, signalingSentCount int) {
	for _, node := range nodes {
		switch node.State {
		case appc.SignalPending:
			pendingSignalCount++
		case appc.SignalSent:
			signalingSentCount++
		}
	}

	return
}
