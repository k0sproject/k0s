//go:build unix

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package k0s

import (
	"testing"

	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"

	"github.com/stretchr/testify/assert"
)

func TestRestartTracker(t *testing.T) {
	newSignalData := func(planID, statusTimestamp string) apsigv2.SignalData {
		commandID := 123
		return apsigv2.SignalData{
			PlanID:  planID,
			Created: "now",
			Command: apsigv2.Command{
				ID: &commandID,
				K0sUpdate: &apsigv2.CommandK0sUpdate{
					URL:         "https://updates.example.com/k0s",
					Version:     "v99.99.99",
					ForceUpdate: true,
				},
			},
			Status: &apsigv2.Status{Status: Restart, Timestamp: statusTimestamp},
		}
	}

	restartSignal := newSignalData("plan1", "2020-01-01T12:30:00Z")

	renewedRestartSignal := restartSignal
	renewedRestartSignal.Status = &apsigv2.Status{Status: Restart, Timestamp: "2020-01-01T12:31:00Z"}

	// Some other plan's signal data that reached the Restart state.
	otherPlanRestartSignal := newSignalData("plan2", "2020-01-01T12:32:00Z")

	t.Run("nothing pending initially", func(t *testing.T) {
		var underTest RestartTracker
		assert.False(t, underTest.IsRestartPending(restartSignal))
	})

	t.Run("pending after initiation", func(t *testing.T) {
		var underTest RestartTracker
		underTest.RestartInitiated(restartSignal)
		assert.True(t, underTest.IsRestartPending(restartSignal))
		assert.False(t, underTest.IsRestartPending(renewedRestartSignal))
		assert.False(t, underTest.IsRestartPending(otherPlanRestartSignal))
	})

	t.Run("latest initiation wins", func(t *testing.T) {
		var underTest RestartTracker
		underTest.RestartInitiated(restartSignal)
		underTest.RestartInitiated(otherPlanRestartSignal)
		assert.False(t, underTest.IsRestartPending(restartSignal))
		assert.True(t, underTest.IsRestartPending(otherPlanRestartSignal))
	})
}
