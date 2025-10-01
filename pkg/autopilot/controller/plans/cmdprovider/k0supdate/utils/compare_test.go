// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"

	"github.com/stretchr/testify/assert"
)

// TestIsSignalDataSameCommand runs tests around the `isSignalDataSameCommand` function.
func TestIsSignalDataSameCommand(t *testing.T) {
	var tests = []struct {
		name       string
		command    apv1beta2.PlanCommandStatus
		signalData apsigv2.SignalData
		same       bool
	}{
		{
			"Same",
			apv1beta2.PlanCommandStatus{
				K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{},
			},
			apsigv2.SignalData{
				Command: apsigv2.Command{
					K0sUpdate: &apsigv2.CommandK0sUpdate{},
				},
			},
			true,
		},
		{
			"NotSameSignalDataNil",
			apv1beta2.PlanCommandStatus{
				K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{},
			},
			apsigv2.SignalData{
				Command: apsigv2.Command{},
			},
			false,
		},
		{
			"NotSameCommandNil",
			apv1beta2.PlanCommandStatus{},
			apsigv2.SignalData{
				Command: apsigv2.Command{
					K0sUpdate: &apsigv2.CommandK0sUpdate{},
				},
			},
			false,
		},
		{
			"NotSameSignalData",
			apv1beta2.PlanCommandStatus{
				K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{},
			},
			apsigv2.SignalData{
				Command: apsigv2.Command{
					AirgapUpdate: &apsigv2.CommandAirgapUpdate{},
				},
			},
			false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.same, IsSignalDataSameCommand(test.command, test.signalData))
		})
	}
}

// TestIsSignalDataStatusDifferent runs tests around the `isSignalDataStatusDifferent` function.
func TestIsSignalDataStatusDifferent(t *testing.T) {
	var tests = []struct {
		name             string
		signalNode       apv1beta2.PlanCommandTargetStatus
		signalDataStatus *apsigv2.Status
		different        bool
	}{
		{
			"Different",
			apv1beta2.PlanCommandTargetStatus{
				State: "foo",
			},
			&apsigv2.Status{
				Status: "oof",
			},
			true,
		},
		{
			"Same",
			apv1beta2.PlanCommandTargetStatus{
				State: "foo",
			},
			&apsigv2.Status{
				Status: "foo",
			},
			false,
		},
		{
			"NilStatus",
			apv1beta2.PlanCommandTargetStatus{
				State: "foo",
			},
			nil,
			false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.different, IsSignalDataStatusDifferent(test.signalNode, test.signalDataStatus))
		})
	}
}
