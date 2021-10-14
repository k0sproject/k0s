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

package k0supdate

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	apv1beta2 "github.com/k0sproject/autopilot/pkg/apis/autopilot.k0sproject.io/v1beta2"
	apdel "github.com/k0sproject/autopilot/pkg/controller/delegate"
	appc "github.com/k0sproject/autopilot/pkg/controller/plans/core"
	apsigv2 "github.com/k0sproject/autopilot/pkg/signaling/v2"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

// Schedulable handles the provider state 'schedulable'
func (kp *k0supdate) Schedulable(ctx context.Context, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
	logger := kp.logger.WithField("state", "schedulable")
	logger.Info("Processing")

	// Once in 'Schedulable', we find the first signal node in 'PendingSignal'. If there
	// are no other candidates, we're considered done.
	//
	// Controllers take priority and are selected before any workers. This implies that
	// all controllers need to be 'up-to-date' in order for any workers to get selected.

	nextForSignal, nextLabel, _ := findNextSchedulableTarget(logger, status.K0sUpdate)
	if nextForSignal == nil {
		// Nothing left to do with this reconciler.
		logger.Infof("All schedulable targets are completed")
		return appc.PlanCompleted, false, nil
	}

	// Lookup the signal node for this next target, and send it signaling

	signalNodeDelegate, ok := kp.controllerDelegateMap[nextLabel]
	if !ok {
		logger.Warnf("Missing signal delegate for '%s'", nextLabel)
		return appc.PlanMissingSignalNode, false, nil
	}

	nodeKey := signalNodeDelegate.CreateNamespacedName(nextForSignal.Name)
	signalNode := signalNodeDelegate.CreateObject()
	if err := kp.client.Get(ctx, nodeKey, signalNode); err != nil {
		logger.Warnf("Unable to find signal node '%s' for signal: %v", nodeKey, err)
		return appc.PlanMissingSignalNode, false, nil
	}

	// If the found signal node is not ready to accept an update, either complete this reconciliation
	// in order to move onto the next signal node candidate, or requeue if this is the last remaining
	// candidate.

	updateReadyStatus := signalNodeDelegate.K0sUpdateReady(*status.K0sUpdate, signalNode)
	if updateReadyStatus != apdel.CanUpdate {
		if updateReadyStatus == apdel.Inconsistent {
			// If we're inconsistent, there is nothing else we can do -- operator intervention
			// is now required.

			logger.Warn("Inconsistent targets detected, unable to process.")
			return appc.PlanInconsistentTargets, false, nil
		}

		// Request a requeue with the current status
		return status.State, true, nil
	}

	logger.Infof("Sending signalling to node='%s'", nextForSignal.Name)

	// Add the signaling instructions to the nodes metadata.
	//
	// This has the possibility of ending reconciliation early if the node and plan platforms
	// disagree. This target state will move to `IncompleteTargets` in this case.

	nodeCopy, err := signalNodeUpdate(signalNodeDelegate.DeepCopy(signalNode), cmd.K0sUpdate, status.K0sUpdate)
	if err != nil {
		logger.Warnf("Unable to update signal node: %v", err)
		return appc.PlanIncompleteTargets, false, nil
	}

	// .. and update the node

	if err := kp.client.Update(ctx, nodeCopy, &crcli.UpdateOptions{}); err != nil {
		logger.Warnf("Unable to update signalnode with signaling: %v", err)
		return status.State, false, fmt.Errorf("unable to update signalnode with signaling: %w", err)
	}

	// Update the status of the node we sent the signal to

	updatePlanCommandTargetStatusByName(nextForSignal.Name, appc.SignalSent, status.K0sUpdate)

	return appc.PlanSchedulableWait, false, nil
}

// findNextSchedulableTarget searches through the plan status targets, searching for the
// first entry that has the status `PendingSignal`. The plan targets are either a 'controller',
// or a 'worker', and have a label indicating this. If none remain, nil is returned.
func findNextSchedulableTarget(logger *logrus.Entry, cmd *apv1beta2.PlanCommandK0sUpdateStatus) (*apv1beta2.PlanCommandTargetStatus, string, int) {
	var targets = []struct {
		nodes []apv1beta2.PlanCommandTargetStatus
		label string
	}{
		{cmd.Controllers, "controller"},
		{cmd.Workers, "worker"},
	}

	for _, target := range targets {
		pendingNodes := findPending(target.nodes)
		pendingNodeCount := len(pendingNodes)

		if pendingNodeCount > 0 {
			nextNode, err := findNextPendingRandom(pendingNodes)
			if err != nil {
				logger.Errorf("Unable to determine next random node: %v", err)
			}

			if nextNode != nil {
				return nextNode, target.label, pendingNodeCount
			}
		}
	}

	return nil, "", 0
}

func findPending(nodes []apv1beta2.PlanCommandTargetStatus) []apv1beta2.PlanCommandTargetStatus {
	var pendingNodes []apv1beta2.PlanCommandTargetStatus

	for _, node := range nodes {
		if node.State == appc.SignalPending {
			pendingNodes = append(pendingNodes, node)
		}
	}

	return pendingNodes
}

// findNextPendingRandom finds a random `PlanCommandTargetStatus` in the provided slice that
// has the status of `PendingSignal`
func findNextPendingRandom(nodes []apv1beta2.PlanCommandTargetStatus) (*apv1beta2.PlanCommandTargetStatus, error) {
	count := int64(len(nodes))
	if count > 0 {
		idx, err := rand.Int(rand.Reader, big.NewInt(count))
		if err != nil {
			return nil, err
		}

		node := nodes[idx.Int64()]
		return &node, nil
	}

	return nil, nil
}

// signalNodeUpdate builds a signalling update request, and adds it to the provided node
func signalNodeUpdate(node crcli.Object, cmd *apv1beta2.PlanCommandK0sUpdate, cmdStatus *apv1beta2.PlanCommandK0sUpdateStatus) (crcli.Object, error) {
	// Determine the platform identifier of the target signal node
	nodePlatformID, err := signalNodePlatformIdentifier(node)
	if err != nil {
		updatePlanCommandTargetStatusByName(node.GetName(), appc.SignalMissingPlatform, cmdStatus)
		return nil, err
	}

	// Find the appropriate update content for this signal node
	updateContent, updateContentOk := cmd.Platforms[nodePlatformID]
	if !updateContentOk {
		updatePlanCommandTargetStatusByName(node.GetName(), appc.SignalMissingPlatform, cmdStatus)
		return nil, err
	}

	signalData := apsigv2.SignalData{
		Created: time.Now().Format(time.RFC3339),
		Command: apsigv2.Command{
			Update: &apsigv2.CommandUpdateItem{
				K0s: &apsigv2.CommandUpdateItemK0s{
					URL:     updateContent.URL,
					Version: cmd.Version,
					Sha256:  updateContent.Sha256,
				},
			},
		},
	}

	if err := signalData.Validate(); err != nil {
		return nil, fmt.Errorf("unable to validate signaling data: %w", err)
	}

	if node.GetAnnotations() == nil {
		node.SetAnnotations(make(map[string]string))
	}

	if err := signalData.Marshal(node.GetAnnotations()); err != nil {
		return nil, fmt.Errorf("unable to marshal signaling data: %w", err)
	}

	return node, nil
}

// UpdatePlanCommandTargetStatusByName searches through nodes in the plan status, updating the
// status for the node with the provided name.
func updatePlanCommandTargetStatusByName(name string, status apv1beta2.PlanCommandTargetStateType, cmdStatus *apv1beta2.PlanCommandK0sUpdateStatus) {
	groups := [][]apv1beta2.PlanCommandTargetStatus{
		cmdStatus.Controllers,
		cmdStatus.Workers,
	}

	for _, group := range groups {
		for idx, node := range group {
			if node.Name == name {
				group[idx].State = status
				group[idx].LastUpdatedTimestamp = metav1.Now()
				return
			}
		}
	}
}
