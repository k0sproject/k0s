//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package k0s

import (
	"testing"

	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"

	"github.com/stretchr/testify/assert"
)

// TestSignalDataUpdateCommandK0sPredicate ensures that the predicate can properly identify
// k0s updates.
func TestSignalDataUpdateCommandK0sPredicate(t *testing.T) {
	var tests = []struct {
		name    string
		data    apsigv2.SignalData
		success bool
	}{
		{
			"Found",
			apsigv2.SignalData{
				Command: apsigv2.Command{
					ID:        new(int),
					K0sUpdate: &apsigv2.CommandK0sUpdate{},
				},
			},
			true,
		},
		{
			"NotFoundAirgap",
			apsigv2.SignalData{
				Command: apsigv2.Command{
					ID:           new(int),
					AirgapUpdate: &apsigv2.CommandAirgapUpdate{},
				},
			},
			false,
		},
		{
			"NotFoundMissingUpdate",
			apsigv2.SignalData{
				Command: apsigv2.Command{},
			},
			false,
		},
	}

	pred := signalDataUpdateCommandK0sPredicate()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.success, pred(test.data))
		})
	}
}
