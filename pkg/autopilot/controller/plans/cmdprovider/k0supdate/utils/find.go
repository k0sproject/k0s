// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"crypto/rand"
	"math/big"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
)

func FindPending(nodes []apv1beta2.PlanCommandTargetStatus) []apv1beta2.PlanCommandTargetStatus {
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
func FindNextPendingRandom(nodes []apv1beta2.PlanCommandTargetStatus) (*apv1beta2.PlanCommandTargetStatus, error) {
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
