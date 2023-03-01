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

package delegate

import (
	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"

	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

// TestNodeReady ensures that the delegate can identify when a worker node is
// in the ready state
func TestNodeReady(t *testing.T) {
	var tests = []struct {
		name          string
		node          *v1.Node
		expectedReady K0sUpdateReadyStatus
	}{
		{
			"NodeReady",
			&v1.Node{
				Status: v1.NodeStatus{
					Conditions: []v1.NodeCondition{
						{Type: v1.NodeReady, Status: v1.ConditionTrue},
					},
				},
			},
			CanUpdate,
		},
		{
			"NodeNotReady",
			&v1.Node{
				Status: v1.NodeStatus{
					Conditions: []v1.NodeCondition{
						{Type: v1.NodeReady, Status: v1.ConditionFalse},
					},
				},
			},
			NotReady,
		},
		{
			"NodeReadyMissing",
			&v1.Node{
				Status: v1.NodeStatus{
					Conditions: []v1.NodeCondition{
						{Type: v1.NodeDiskPressure, Status: v1.ConditionFalse},
					},
				},
			},
			NotReady,
		},
	}

	delegate := NodeControllerDelegate()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expectedReady, delegate.K0sUpdateReady(apv1beta2.PlanCommandK0sUpdateStatus{}, test.node))
		})
	}
}
