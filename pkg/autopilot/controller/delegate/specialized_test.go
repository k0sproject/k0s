// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package delegate

import (
	"encoding/json"
	"strings"
	"testing"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
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

func TestReadSignalError(t *testing.T) {
	var tests = []struct {
		name     string
		obj      crcli.Object
		delegate ControllerDelegate
		expected string
	}{
		{
			"ControlNodeWithErrorAnnotation",
			&apv1beta2.ControlNode{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						SignalErrorAnnotation: `{"planID":"plan1","reason":"FailedDownload","message":"checksum mismatch","timestamp":"2024-01-01T00:00:00Z"}`,
					},
				},
			},
			ControlNodeControllerDelegate(),
			"FailedDownload: checksum mismatch",
		},
		{
			"ControlNodeNoAnnotation",
			&apv1beta2.ControlNode{},
			ControlNodeControllerDelegate(),
			"",
		},
		{
			"NodeWithErrorAnnotation",
			&v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						SignalErrorAnnotation: `{"planID":"plan1","reason":"FailedDownload","message":"checksum mismatch","timestamp":"2024-01-01T00:00:00Z"}`,
					},
				},
			},
			NodeControllerDelegate(),
			"FailedDownload: checksum mismatch",
		},
		{
			"NodeNoAnnotation",
			&v1.Node{},
			NodeControllerDelegate(),
			"",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, test.delegate.ReadSignalError(test.obj))
		})
	}
}

func TestTruncate(t *testing.T) {
	var tests = []struct {
		name     string
		input    string
		max      int
		expected string
	}{
		{"BelowLimit", "short", 10, "short"},
		{"AtLimit", "exactly10!", 10, "exactly10!"},
		{"OneOverLimit", "exactly10!x", 10, "exactly..."},
		{"WellOverLimit", strings.Repeat("a", 100), 10, strings.Repeat("a", 7) + "..."},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := truncate(test.input, test.max)
			assert.Equal(t, test.expected, got)
			assert.LessOrEqual(t, len(got), test.max)
		})
	}
}

func TestWriteSignalErrorAnnotationTruncation(t *testing.T) {
	longMessage := strings.Repeat("m", maxMessageLen+10)

	for _, tc := range []struct {
		name     string
		obj      crcli.Object
		delegate ControllerDelegate
	}{
		{"ControlNode", &apv1beta2.ControlNode{}, ControlNodeControllerDelegate()},
		{"Node", &v1.Node{}, NodeControllerDelegate()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.delegate.WriteSignalError(t.Context(), nil, tc.obj, "plan1", "FailedDownload", longMessage)
			assert.NoError(t, err)

			raw, ok := tc.obj.GetAnnotations()[SignalErrorAnnotation]
			assert.True(t, ok, "annotation should be set")

			var e SignalError
			assert.NoError(t, json.Unmarshal([]byte(raw), &e))
			assert.Equal(t, "FailedDownload", e.Reason, "reason should be stored as-is")
			assert.LessOrEqual(t, len(e.Message), maxMessageLen, "message should be truncated")
			assert.True(t, strings.HasSuffix(e.Message, "..."), "truncated message should end with ...")
		})
	}
}

func TestClearSignalErrorAnnotation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		obj      crcli.Object
		delegate ControllerDelegate
	}{
		{"ControlNode", &apv1beta2.ControlNode{ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{SignalErrorAnnotation: `{"planID":"p","reason":"r","message":"m","timestamp":"2024-01-01T00:00:00Z"}`},
		}}, ControlNodeControllerDelegate()},
		{"Node", &v1.Node{ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{SignalErrorAnnotation: `{"planID":"p","reason":"r","message":"m","timestamp":"2024-01-01T00:00:00Z"}`},
		}}, NodeControllerDelegate()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.delegate.ClearSignalError(t.Context(), nil, tc.obj)
			_, ok := tc.obj.GetAnnotations()[SignalErrorAnnotation]
			assert.False(t, ok, "annotation should be cleared")
		})
	}
}
