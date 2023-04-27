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
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	appku "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/cmdprovider/k0supdate/utils"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	apsigcomm "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"
)

// SchedulableWait handles the provider state 'schedulablewait'
func (aup *airgapupdate) SchedulableWait(ctx context.Context, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
	logger := aup.logger.WithField("state", "schedulablewait")
	logger.Info("Processing")

	// Update the target status for both controllers and workers based on queries
	// of their respective signal node objects.

	logger.Info("Reconciling controller/worker signal node statuses")
	aup.reconcileSignalNodeStatusTarget(ctx, planID, *status, aup.controllerDelegateMap["worker"], status.AirgapUpdate.Workers)

	// If any of the nodes have reported a failure in applying an update, the
	// plan is marked as a failure.

	if appku.IsNotRecoverable(status.AirgapUpdate.Workers) {
		logger.Info("Plan is non-recoverable due to apply failure")
		return appc.PlanApplyFailed, false, nil
	}

	if appku.IsCompleted(status.AirgapUpdate.Workers) {
		logger.Info("Workers completed")
		return appc.PlanCompleted, false, nil
	}

	canScheduleWorkers, _ := isSchedulableWorkers(cmd.AirgapUpdate.Workers, status.AirgapUpdate.Workers)

	if canScheduleWorkers {
		logger.Info("Workers can be scheduled (controllers done)")
		return appc.PlanSchedulable, false, nil
	}

	logger.Info("No applicable transitions available, requesting retry")
	return appc.PlanSchedulableWait, true, nil
}

// reconcileSignalNodeStatusTarget performs a reconciliation of the status of every signal node provided
// against the current state maintained in the plan status. This ensures that any signal nodes that
// have been transitioned to 'Completed' will also appear in the plan status as 'Completed'.
func (aup *airgapupdate) reconcileSignalNodeStatusTarget(ctx context.Context, planID string, cmdStatus apv1beta2.PlanCommandStatus, delegate apdel.ControllerDelegate, signalNodes []apv1beta2.PlanCommandTargetStatus) {
	for i := 0; i < len(signalNodes); i++ {
		if signalNodes[i].State == appc.SignalCompleted {
			continue
		}

		key := delegate.CreateNamespacedName(signalNodes[i].Name)
		signalNode := delegate.CreateObject()

		if err := aup.client.Get(ctx, key, signalNode); err != nil {
			aup.logger.Warnf("Unable to find signal node '%s'", signalNodes[i].Name)
			continue
		}

		if apsigv2.IsSignalingPresent(signalNode.GetAnnotations()) {
			var signalData apsigv2.SignalData
			if err := signalData.Unmarshal(signalNode.GetAnnotations()); err == nil {
				if signalData.PlanID == planID {
					// Ensure that the commands are the same, but their status's are different before we check completed.
					if appku.IsSignalDataSameCommand(cmdStatus, signalData) && appku.IsSignalDataStatusDifferent(signalNodes[i], signalData.Status) {
						origState := signalNodes[i].State

						if signalData.Status.Status == apsigcomm.Failed || signalData.Status.Status == apsigcomm.FailedDownload {
							signalNodes[i].State = appc.SignalApplyFailed
						}

						if signalData.Status.Status == apsigcomm.Completed {
							signalNodes[i].State = appc.SignalCompleted
						}

						aup.logger.Infof("Signal node '%s' status changed from '%s' to '%s' (reason: %s)", signalNodes[i].Name, origState, signalNodes[i].State, signalData.Status.Status)
					}
				} else {
					aup.logger.Warnf("Current planid '%v' doesn't match signal node planid '%v'", planID, signalData.PlanID)
				}
			} else {
				aup.logger.Warnf("Unable to unmarshal signaling data from signal node '%s'", signalNode.GetName())
			}
		}
	}
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
