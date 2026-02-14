// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v2

import (
	"maps"
	"slices"
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
			if test.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
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
			if test.successful {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
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
			if test.successful {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
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
	mapKeys := slices.Collect(maps.Keys(m))
	assert.ElementsMatch(t, []string{
		"k0sproject.io/autopilot-signal-version",
		"k0sproject.io/autopilot-signal-data",
	}, mapKeys)

	// .. and backward
	signalData2 := SignalData{}
	err := signalData2.Unmarshal(m)
	assert.NoError(t, err)

	assert.Equal(t, signalData1, signalData2)
}
