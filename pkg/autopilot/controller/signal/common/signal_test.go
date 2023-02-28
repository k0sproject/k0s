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

package common

import (
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// TestExtractSignalData runs through a table of different 'annotation' input
// to cover all of the edge cases in `extractSignalData()`
func TestExtractSignalData(t *testing.T) {
	var tests = []struct {
		name  string
		data  map[string]string
		found bool
	}{
		// Ensures that a map that doesn't validate is invalid.
		{
			"Invalid request",
			map[string]string{},
			false,
		},

		// Ensures that a map that doesn't have an associated status is returned.
		{
			"No status present",
			map[string]string{
				"k0sproject.io/autopilot-signal-version": "v2",
				"k0sproject.io/autopilot-signal-data": `
					{
						"planId":"abc123",
						"created":"now",
						"command": {
							"id": 123,
							"k0supdate": {
								"version": "v1.2.3",
								"url": "https://www.google.com/download.tar.gz",
								"sha256": "thisisthesha"
							}
						}
					}
				`,
			},
			true,
		},

		// Ensures that a map that has a response with an 'Completed' response is considered done,
		// and doesn't result in the request being returned.
		{
			"Ignore Completed",
			map[string]string{
				"k0sproject.io/autopilot-signal-version": "v2",
				"k0sproject.io/autopilot-signal-data": `
					{
						"planId":"abc123",
						"created":"now",
						"command": {
							"id": 123,
							"k0supdate": {
								"version": "v1.2.3",
								"url": "https://www.google.com/download.tar.gz",
								"sha256": "thisisthesha"
							}
						},
						"status": {
							"status": "Completed",
							"timestamp": "now"
						}
					}
				`,
			},
			false,
		},
	}

	logger := logrus.NewEntry(logrus.StandardLogger())

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			signalData := extractSignalData(logger, test.data)
			assert.Equal(t, test.found, (signalData != nil), fmt.Sprintf("Failure in '%s'", test.name))
		})
	}
}
