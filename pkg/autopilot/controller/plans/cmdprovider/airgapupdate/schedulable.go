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
	"fmt"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	appku "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/cmdprovider/k0supdate/utils"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"

	"github.com/sirupsen/logrus"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

// Schedulable handles the provider state 'schedulable'
func (aup *airgapupdate) Schedulable(ctx context.Context, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
	logger := aup.logger.WithField("state", "schedulable")
	logger.Info("Processing")

	// Once in 'Schedulable', we find the first signal node in 'PendingSignal'. If there
	// are no other candidates, we're considered done.
	//
	// Controllers take priority and are selected before any workers. This implies that
	// all controllers need to be 'up-to-date' in order for any workers to get selected.

	nextForSignal := findNextSchedulableTarget(logger, status.AirgapUpdate)
	if nextForSignal == nil {
		// Nothing left to do with this reconciler.
		logger.Infof("All schedulable targets are completed")
		return appc.PlanCompleted, false, nil
	}

	signalNodeDelegate, ok := aup.controllerDelegateMap["worker"]
	if !ok {
		logger.Warnf("Missing signal delegate for '%s'", "worker")
		return appc.PlanMissingSignalNode, false, nil
	}

	nodeKey := signalNodeDelegate.CreateNamespacedName(nextForSignal.Name)
	signalNode := signalNodeDelegate.CreateObject()
	if err := aup.client.Get(ctx, nodeKey, signalNode); err != nil {
		logger.Warnf("Unable to find signal node '%s' for signal: %v", nodeKey, err)
		return appc.PlanMissingSignalNode, false, nil
	}

	logger.Infof("Sending signalling to node='%s'", nextForSignal.Name)

	signalNodeCopy := signalNodeDelegate.DeepCopy(signalNode)
	signalNodeCommandBuilder, err := signalNodeAirgapUpdateCommandBuilder(signalNodeCopy, cmd, status)
	if err != nil {
		logger.Warnf("Unable to build signal node content: %v", err)
		return appc.PlanIncompleteTargets, false, nil
	}

	if err := appku.UpdateSignalNode(signalNodeCopy, planID, signalNodeCommandBuilder); err != nil {
		logger.Warnf("Unable to update signal node: %v", err)
		return appc.PlanIncompleteTargets, false, nil
	}

	// .. and update the node

	if err := aup.client.Update(ctx, signalNodeCopy, &crcli.UpdateOptions{}); err != nil {
		logger.Warnf("Unable to update signalnode with signaling: %v", err)
		return status.State, false, fmt.Errorf("unable to update signalnode with signaling: %w", err)
	}

	// Update the status of the node we sent the signal to

	appku.UpdatePlanCommandTargetStatusByName(nextForSignal.Name, appc.SignalSent, status.AirgapUpdate.Workers)

	return appc.PlanSchedulableWait, false, nil
}

// findNextSchedulableTarget searches through the plan status targets, searching for the
// first entry that has the status `PendingSignal`. The plan targets are either a 'controller',
// or a 'worker', and have a label indicating this. If none remain, nil is returned.
func findNextSchedulableTarget(logger *logrus.Entry, cmd *apv1beta2.PlanCommandAirgapUpdateStatus) *apv1beta2.PlanCommandTargetStatus {
	pendingNodes := appku.FindPending(cmd.Workers)
	pendingNodeCount := len(pendingNodes)

	if pendingNodeCount > 0 {
		nextNode, err := appku.FindNextPendingRandom(pendingNodes)
		if err != nil {
			logger.Errorf("Unable to determine next random node: %v", err)
		}

		if nextNode != nil {
			return nextNode
		}
	}

	return nil
}

func signalNodeAirgapUpdateCommandBuilder(node crcli.Object, cmd apv1beta2.PlanCommand, cmdStatus *apv1beta2.PlanCommandStatus) (appku.SignalNodeCommandBuilder, error) {
	// Determine the platform identifier of the target signal node
	nodePlatformID, err := appku.SignalNodePlatformIdentifier(node)
	if err != nil {
		appku.UpdatePlanCommandTargetStatusByName(node.GetName(), appc.SignalMissingPlatform, cmdStatus.AirgapUpdate.Workers)
		return nil, err
	}

	updateContent, updateContentOk := cmd.AirgapUpdate.Platforms[nodePlatformID]
	if !updateContentOk {
		appku.UpdatePlanCommandTargetStatusByName(node.GetName(), appc.SignalMissingPlatform, cmdStatus.AirgapUpdate.Workers)
		return nil, fmt.Errorf("for platform ID %s: %s", nodePlatformID, appc.SignalMissingPlatform)
	}

	return func() apsigv2.Command {
		return apsigv2.Command{
			ID: &cmdStatus.ID,
			AirgapUpdate: &apsigv2.CommandAirgapUpdate{
				URL:     updateContent.URL,
				Version: cmd.AirgapUpdate.Version,
				Sha256:  updateContent.Sha256,
			},
		}
	}, nil
}
