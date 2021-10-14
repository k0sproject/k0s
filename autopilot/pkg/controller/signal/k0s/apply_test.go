// Copyright 2022 k0s authors
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

package k0s

import (
	"fmt"
	"testing"

	apv1beta2 "github.com/k0sproject/autopilot/pkg/apis/autopilot.k0sproject.io/v1beta2"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crev "sigs.k8s.io/controller-runtime/pkg/event"
)

// TestApplyingUpdateEventFilter runs through a table of scenarios ensuring that
// the event-filtering for the 'applying-update' controller works accordingly.
func TestApplyingUpdateEventFilter(t *testing.T) {
	var tests = []struct {
		name    string
		event   crev.UpdateEvent
		success bool
	}{
		{
			"Happy",
			crev.UpdateEvent{
				ObjectOld: &apv1beta2.ControlNode{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
					},
				},
				ObjectNew: &apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node0",
						Annotations: map[string]string{
							"k0sproject.io/autopilot-signal-version": "v2",
							"k0sproject.io/autopilot-signal-data": `
								{
									"planId":"abc123",
									"created":"now",
									"command": {
										"update": {
											"k0s": {
												"version": "v1.23.3+k0s.0",
												"url": "https://github.com/k0sproject/k0s/releases/download/v1.23.3%2Bk0s.0/k0s-v1.23.3+k0s.0-amd64",
												"timestamp": "2021-10-20T19:06:56Z",
												"sha256": "aa170c7fa0ea3fe1194eaec6a18964543e1e139eab1cfbbbafec7f357fb1679d"
											}
										}
									},
									"status": {
										"status": "ApplyingUpdate",
										"timestamp": "2021-10-20T19:09:11Z"
									}
								}
							`,
						},
					},
				},
			},
			true,
		},
		{
			"Wrong response",
			crev.UpdateEvent{
				ObjectOld: &apv1beta2.ControlNode{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
					},
				},
				ObjectNew: &apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node0",
						Annotations: map[string]string{
							"k0sproject.io/autopilot-signal-version": "v2",
							"k0sproject.io/autopilot-signal-data": `
								{
									"planId":"abc123",
									"created":"now",
									"command": {
										"update": {
											"k0s": {
												"version": "v1.23.3+k0s.0",
												"url": "https://github.com/k0sproject/k0s/releases/download/v1.23.3%2Bk0s.0/k0s-v1.23.3+k0s.0-amd64",
												"timestamp": "2021-10-20T19:06:56Z",
												"sha256": "aa170c7fa0ea3fe1194eaec6a18964543e1e139eab1cfbbbafec7f357fb1679d"
											}
										}
									},
									"status": {
										"status": "Completed",
										"timestamp": "2021-10-20T19:09:11Z"
									}
								}
							`,
						},
					},
				},
			},
			false,
		},
		{
			"No change in annotations",
			crev.UpdateEvent{
				ObjectOld: &apv1beta2.ControlNode{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"k0sproject.io/autopilot-version":                "v1beta2",
							"k0sproject.io/autopilot-command":                "update",
							"k0sproject.io/autopilot-command-id":             "0396fbc9fe1b",
							"k0sproject.io/autopilot-command-timestamp":      "2021-10-20T19:06:56Z",
							"k0sproject.io/autopilot-command-update-sha256":  "thisisthesha",
							"k0sproject.io/autopilot-command-update-url":     "https://www.google.com/download.tar.gz",
							"k0sproject.io/autopilot-command-update-version": "v1.2.3",
							"k0sproject.io/autopilot-plan-id":                "id123",
							"k0sproject.io/autopilot-response":               "Completed",
							"k0sproject.io/autopilot-response-id":            "0396fbc9fe1b",
							"k0sproject.io/autopilot-response-timestamp":     "2021-10-20T19:09:11Z",
						}},
				},
				ObjectNew: &apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node0",
						Annotations: map[string]string{
							"k0sproject.io/autopilot-version":                "v1beta2",
							"k0sproject.io/autopilot-command":                "update",
							"k0sproject.io/autopilot-command-id":             "0396fbc9fe1b",
							"k0sproject.io/autopilot-command-timestamp":      "2021-10-20T19:06:56Z",
							"k0sproject.io/autopilot-command-update-sha256":  "thisisthesha",
							"k0sproject.io/autopilot-command-update-url":     "https://www.google.com/download.tar.gz",
							"k0sproject.io/autopilot-command-update-version": "v1.2.3",
							"k0sproject.io/autopilot-plan-id":                "id123",
							"k0sproject.io/autopilot-response":               "Completed",
							"k0sproject.io/autopilot-response-id":            "0396fbc9fe1b",
							"k0sproject.io/autopilot-response-timestamp":     "2021-10-20T19:09:11Z",
						},
					},
				},
			},
			false,
		},
		{
			"Different hostname",
			crev.UpdateEvent{
				ObjectOld: &apv1beta2.ControlNode{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
					},
				},
				ObjectNew: &apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "nodeDIFFERENT",
						Annotations: map[string]string{
							"k0sproject.io/autopilot-version":                "v1beta2",
							"k0sproject.io/autopilot-command":                "update",
							"k0sproject.io/autopilot-command-id":             "0396fbc9fe1b",
							"k0sproject.io/autopilot-command-timestamp":      "2021-10-20T19:06:56Z",
							"k0sproject.io/autopilot-command-update-sha256":  "thisisthesha",
							"k0sproject.io/autopilot-command-update-url":     "https://www.google.com/download.tar.gz",
							"k0sproject.io/autopilot-command-update-version": "v1.2.3",
							"k0sproject.io/autopilot-plan-id":                "id123",
							"k0sproject.io/autopilot-response":               "Completed",
							"k0sproject.io/autopilot-response-id":            "0396fbc9fe1b",
							"k0sproject.io/autopilot-response-timestamp":     "2021-10-20T19:09:11Z",
						},
					},
				},
			},
			false,
		},
	}

	pred := applyingUpdateEventFilter("node0", func(err error) bool {
		return false
	})

	for idx, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.success, pred.Update(test.event), fmt.Sprintf("Failed in #%d '%s'", idx, test.name))
		})
	}
}
