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

	for i := 0; i < 1000; i++ {
		node, err := FindNextPendingRandom(nodes)
		assert.NoError(t, err)
		assert.NotNil(t, node)

		countMap[node.Name] = countMap[node.Name] + 1
	}

	assert.Contains(t, countMap, "bbb")
	assert.Contains(t, countMap, "ddd")
	assert.Contains(t, countMap, "eee")
	assert.NotEqual(t, 1000, countMap["bbb"])
	assert.NotEqual(t, 1000, countMap["ddd"])
	assert.NotEqual(t, 1000, countMap["eee"])
}
