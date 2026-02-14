// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	"github.com/stretchr/testify/assert"
)

// TestFindNextPendingRandom ensures that we can find a random PlanCommandTargetStatus
// that is in the `PendingStatus` state.
func TestFindNextPendingRandom(t *testing.T) {
	nodes := []apv1beta2.PlanCommandTargetStatus{
		apv1beta2.NewPlanCommandTargetStatus("aaa", appc.SignalCompleted),
		apv1beta2.NewPlanCommandTargetStatus("bbb", appc.SignalPending),
		apv1beta2.NewPlanCommandTargetStatus("ccc", appc.SignalCompleted),
		apv1beta2.NewPlanCommandTargetStatus("ddd", appc.SignalPending),
		apv1beta2.NewPlanCommandTargetStatus("eee", appc.SignalPending),
	}

	countMap := make(map[string]int)

	// Run through the random function a number of times and assert that
	// randomness is working. Its possible that one node can be chosen
	// 100% of the time, but its unlikely.

	for range 1000 {
		node, err := FindNextPendingRandom(nodes)
		assert.NoError(t, err)
		assert.NotNil(t, node)

		countMap[node.Name] += 1
	}

	assert.Contains(t, countMap, "bbb")
	assert.Contains(t, countMap, "ddd")
	assert.Contains(t, countMap, "eee")
	assert.NotEqual(t, 1000, countMap["bbb"])
	assert.NotEqual(t, 1000, countMap["ddd"])
	assert.NotEqual(t, 1000, countMap["eee"])
}
