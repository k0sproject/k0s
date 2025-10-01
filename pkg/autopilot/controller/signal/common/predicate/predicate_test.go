// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package predicate

import (
	"testing"

	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"

	"github.com/stretchr/testify/assert"
)

// TestDataStatusPredicate tests the scenarios of matching a status in SignalData
func TestDataStatusPredicate(t *testing.T) {
	var tests = []struct {
		name    string
		pred    SignalDataPredicate
		data    apsigv2.SignalData
		success bool
	}{
		{
			"Matches",
			SignalDataStatusPredicate("foo"),
			apsigv2.SignalData{Status: &apsigv2.Status{Status: "foo"}},
			true,
		},
		{
			"NoMatchInSignalData",
			SignalDataStatusPredicate("foo"),
			apsigv2.SignalData{Status: &apsigv2.Status{Status: "bar"}},
			false,
		},
		{
			"NoMatchInSignalDataStatusMissing",
			SignalDataStatusPredicate("foo"),
			apsigv2.SignalData{},
			false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.success, test.pred(test.data))
		})
	}
}

// TestDataNoStatusPredicate tests the scenarios of identifying if a SignalData has status.
func TestDataNoStatusPredicate(t *testing.T) {
	var tests = []struct {
		name    string
		data    apsigv2.SignalData
		success bool
	}{
		{"WithStatus", apsigv2.SignalData{Status: &apsigv2.Status{Status: "with"}}, false},
		{"WithoutStatus", apsigv2.SignalData{}, true},
	}

	pred := SignalDataNoStatusPredicate()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.success, pred(test.data))
		})
	}
}
