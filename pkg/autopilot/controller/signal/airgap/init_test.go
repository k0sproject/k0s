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

package airgap

import (
	"testing"

	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"

	"github.com/stretchr/testify/assert"
)

// TestSignalDataUpdateCommandAirgapPredicate ensures that the predicate can properly identify
// airgap updates.
func TestSignalDataUpdateCommandAirgapPredicate(t *testing.T) {
	var tests = []struct {
		name    string
		data    apsigv2.SignalData
		success bool
	}{
		{
			"Found",
			apsigv2.SignalData{
				Command: apsigv2.Command{
					ID:           new(int),
					AirgapUpdate: &apsigv2.CommandAirgapUpdate{},
				},
			},
			true,
		},
		{
			"NotFoundK0s",
			apsigv2.SignalData{
				Command: apsigv2.Command{
					ID:        new(int),
					K0sUpdate: &apsigv2.CommandK0sUpdate{},
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

	pred := signalDataUpdateCommandAirgapPredicate()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.success, pred(test.data), "Failed in '%s'", test.name)
		})
	}
}
