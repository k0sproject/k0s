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

package utils

import (
	"context"
	"testing"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	apscheme "github.com/k0sproject/k0s/pkg/client/clientset/scheme"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestObjectExistsWithPlatform runs through a table of different ControlNode/Node configurations
// testing `objectExistsWithPlatform`
func TestObjectExistsWithPlatform(t *testing.T) {
	var tests = []struct {
		name           string
		objectName     string
		object         crcli.Object
		objects        []crcli.Object
		plan           apv1beta2.PlanPlatformResourceURLMap
		expectedFound  bool
		expectedStatus *apv1beta2.PlanCommandTargetStateType
	}{
		{
			"HappyControlNode",
			"controller0",
			&apv1beta2.ControlNode{},
			[]crcli.Object{
				&apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:   "controller0",
						Labels: map[string]string{corev1.LabelOSStable: "theOS", corev1.LabelArchStable: "theArch"},
					},
				},
			},
			apv1beta2.PlanPlatformResourceURLMap{
				"theOS-theArch": apv1beta2.PlanResourceURL{}, // just needs to exist
			},
			true,
			nil,
		},
		{
			"ControlNodeMissingPlatformNode",
			"controller0",
			&apv1beta2.ControlNode{},
			[]crcli.Object{
				&apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "controller0",
					},
				},
			},
			apv1beta2.PlanPlatformResourceURLMap{
				"theOS-theArch": apv1beta2.PlanResourceURL{}, // just needs to exist
			},
			false,
			&appc.SignalMissingPlatform,
		},
		{
			"ControlNodeMissingPlatformPlan",
			"controller0",
			&apv1beta2.ControlNode{},
			[]crcli.Object{
				&apv1beta2.ControlNode{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ControlNode",
						APIVersion: "autopilot.k0sproject.io/v1beta2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:   "controller0",
						Labels: map[string]string{corev1.LabelOSStable: "theOS", corev1.LabelArchStable: "theArch"},
					},
				},
			},
			apv1beta2.PlanPlatformResourceURLMap{
				// intentionally empty
			},
			false,
			&appc.SignalMissingPlatform,
		},
		{
			"HappyNode",
			"worker0",
			&corev1.Node{},
			[]crcli.Object{
				&corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:   "worker0",
						Labels: map[string]string{corev1.LabelOSStable: "theOS", corev1.LabelArchStable: "theArch"},
					},
				},
			},
			apv1beta2.PlanPlatformResourceURLMap{
				"theOS-theArch": apv1beta2.PlanResourceURL{}, // just needs to exist
			},
			true,
			nil,
		},
		{
			"NodeMissingPlatformNode",
			"worker0",
			&corev1.Node{},
			[]crcli.Object{
				&corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker0",
					},
				},
			},
			apv1beta2.PlanPlatformResourceURLMap{
				"theOS-theArch": apv1beta2.PlanResourceURL{}, // just needs to exist
			},
			false,
			&appc.SignalMissingPlatform,
		},
		{
			"NodeMissingPlatformPlan",
			"worker0",
			&corev1.Node{},
			[]crcli.Object{
				&corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:   "worker0",
						Labels: map[string]string{corev1.LabelOSStable: "theOS", corev1.LabelArchStable: "theArch"},
					},
				},
			},
			apv1beta2.PlanPlatformResourceURLMap{
				// intentionally empty
			},
			false,
			&appc.SignalMissingPlatform,
		},
	}

	scheme := runtime.NewScheme()
	assert.NoError(t, apscheme.AddToScheme(scheme))
	assert.NoError(t, corev1.AddToScheme(scheme))

	for _, test := range tests {
		client := crfake.NewClientBuilder().WithObjects(test.objects...).WithScheme(scheme).Build()

		t.Run(test.name, func(t *testing.T) {
			found, status := ObjectExistsWithPlatform(context.TODO(), client, test.objectName, test.object, test.plan)
			assert.Equal(t, test.expectedFound, found)
			assert.Equal(t, test.expectedStatus, status)
		})
	}
}
