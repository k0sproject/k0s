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

package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSignalValid tests the validation of the direct fields in `Signal`
func TestSignalValid(t *testing.T) {
	var tests = []struct {
		name   string
		signal Signal
		valid  bool
	}{
		{"GoodVersion", Signal{Version: "v2", Data: "foo"}, true},
		{"BadVersion", Signal{Version: "v1", Data: "foo"}, false},
		{"DataMissing", Signal{Version: "v2", Data: ""}, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.signal.Validate()
			assert.Equal(t, test.valid, err == nil, "Test '%s': validate failed - %v", test.name, err)
		})
	}
}

// TestSignalDataValid tests the validation of the direct fields in `SignalData`.
func TestSignalDataValid(t *testing.T) {
	commandK0s := Command{
		ID: new(int),
		K0sUpdate: &CommandK0sUpdate{
			URL:     "https://foo.bar.baz",
			Version: "v1.2.3",
		},
	}

	status := &Status{"something", "now"}

	var tests = []struct {
		name       string
		data       SignalData
		successful bool
	}{
		// K0s data
		{"Happy", SignalData{"id123", "now", commandK0s, status}, true},
		{"MissingPlanID", SignalData{"", "now", commandK0s, status}, false},
		{"MissingTimestamp", SignalData{"id123", "", commandK0s, status}, false},
		{"MissingStatus", SignalData{"id123", "now", commandK0s, nil}, true},
		{"MissingCommand", SignalData{"id123", "now", Command{}, status}, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.data.Validate()
			assert.Equal(t, test.successful, err == nil, "Test '%s': validate failed - %v", test.name, err)
		})
	}
}

// TestSignalDataUpdateK0sValid tests the validation of `CommandUpdateItemK0s` entries
// in a `CommandUpdate`.
func TestSignalDataUpdateK0sValid(t *testing.T) {
	makeSignalData := func(url, version, sha256 string) SignalData {
		return SignalData{
			PlanID:  "id123",
			Created: "now",
			Command: Command{
				ID: new(int),
				K0sUpdate: &CommandK0sUpdate{
					URL:     url,
					Version: version,
					Sha256:  sha256,
				},
			},
			Status: &Status{
				Status:    "something",
				Timestamp: "now",
			},
		}
	}

	var tests = []struct {
		name       string
		data       SignalData
		successful bool
	}{
		{
			"Happy",
			makeSignalData("https://foo.bar.baz", "v1.2.3", "deadbeef"),
			true,
		},
		{
			"MissingUrl",
			makeSignalData("", "v1.2.3", "deadbeef"),
			false,
		},
		{
			"MissingVersion",
			makeSignalData("https://foo.bar.baz", "", "deadbeef"),
			false,
		},
		{
			"MissingSha256",
			makeSignalData("https://foo.bar.baz", "v1.2.3", ""),
			true, // a missing SHA256 is okay (optional)
		},
		{
			"K0sRequired",
			SignalData{
				PlanID:  "id123",
				Created: "now",
				Command: Command{
					ID:        new(int),
					K0sUpdate: &CommandK0sUpdate{},
				},
			},
			false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.data.Validate()
			assert.Equal(t, test.successful, err == nil, "Test '%s': validate failed - %v", test.name, err)
		})
	}
}

func TestMarshaling(t *testing.T) {
	signalData1 := SignalData{
		PlanID:  "id123",
		Created: "now",
		Command: Command{
			ID: new(int),
			K0sUpdate: &CommandK0sUpdate{
				URL:     "https://foo.bar.baz",
				Version: "v1.2.3",
				Sha256:  "deadbeef",
			},
		},
		Status: &Status{
			Status:    "something",
			Timestamp: "now",
		},
	}

	m := make(map[string]string)
	assert.Empty(t, m)

	// Forward ..
	assert.NoError(t, signalData1.Marshal(m))
	assert.NotEmpty(t, m)
	assert.Equal(t, 2, len(m))
	assert.Contains(t, m, "k0sproject.io/autopilot-signal-version")
	assert.Contains(t, m, "k0sproject.io/autopilot-signal-data")

	// .. and backward
	signalData2 := SignalData{}
	err := signalData2.Unmarshal(m)
	assert.NoError(t, err)

	assert.Equal(t, signalData1, signalData2)
}
