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
			assert.Equal(t, test.success, test.pred(test.data), "Failed on '%s'", test.name)
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
			assert.Equal(t, test.success, pred(test.data), "Failed on '%s'", test.name)
		})
	}
}
