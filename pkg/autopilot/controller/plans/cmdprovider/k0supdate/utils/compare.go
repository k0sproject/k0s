// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"
)

// isSignalDataSameCommand determines if the `PlanCommand` and the command specified in the signal data represent
// the same command.
func IsSignalDataSameCommand(cmdStatus apv1beta2.PlanCommandStatus, signalData apsigv2.SignalData) bool {

	// As additional commands are implemented, they will need to be reflected here.

	switch {
	case cmdStatus.K0sUpdate != nil:
		return signalData.Command.K0sUpdate != nil
	case cmdStatus.AirgapUpdate != nil:
		return signalData.Command.AirgapUpdate != nil
	}

	return false
}

// isSignalDataStatusDifferent determines if the signal node status and the signaling status have different
// status values.
func IsSignalDataStatusDifferent(signalNode apv1beta2.PlanCommandTargetStatus, signalDataStatus *apsigv2.Status) bool {
	return signalDataStatus != nil && signalDataStatus.Status != signalNode.State.String()
}

// IsCompleted determines if every PlanCommandTargetStatus is marked as 'completed'.
func IsCompleted(targets []apv1beta2.PlanCommandTargetStatus) bool {
	for _, target := range targets {
		if target.State != appc.SignalCompleted {
			return false
		}
	}

	return true
}

// IsNotRecoverable determines if any of the PlanCommandTargetStatus is considered non-recoverable.
func IsNotRecoverable(groups ...[]apv1beta2.PlanCommandTargetStatus) bool {
	for _, group := range groups {
		for _, target := range group {
			if target.State == appc.SignalApplyFailed {
				return true
			}
		}
	}

	return false
}
