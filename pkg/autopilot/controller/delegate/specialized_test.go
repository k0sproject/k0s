// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
			assert.Equal(t, test.expectedReady, delegate.K0sUpdateReady(t.Context(), apv1beta2.PlanCommandK0sUpdateStatus{}, test.node))
		})
	}
}
