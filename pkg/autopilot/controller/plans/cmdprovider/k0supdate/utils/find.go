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
