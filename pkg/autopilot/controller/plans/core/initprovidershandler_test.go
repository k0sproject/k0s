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

package core

import (
	"context"
	"fmt"
	"testing"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apscheme2 "github.com/k0sproject/k0s/pkg/client/clientset/scheme"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TestInitProvidersHandle runs through a table of scenarios for testing `initprovidershandler`
func TestInitProvidersHandle(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())

	var tests = []struct {
		name               string
		plan               *apv1beta2.Plan
		handler            PlanStateHandler
		expectedResult     ProviderResult
		expectedError      bool
		expectedPlanStatus *apv1beta2.PlanStatus
	}{
		// A happy-path scenario that drills down to the adapter, confirming status
		{
			"HappyK0sUpdate",
			&apv1beta2.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "Happy",
				},
				Spec: apv1beta2.PlanSpec{
					Commands: []apv1beta2.PlanCommand{
						{
							K0sUpdate: &apv1beta2.PlanCommandK0sUpdate{
								Version: "v1.2.3",
							},
						},
						{
							K0sUpdate: &apv1beta2.PlanCommandK0sUpdate{
								Version: "v4.5.6",
							},
						},
					},
				},
				Status: apv1beta2.PlanStatus{
					State: PlanSchedulable,
				},
			},
			NewInitProvidersHandler(
				logger,
				func(ctx context.Context, provider PlanCommandProvider, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
					return provider.NewPlan(ctx, cmd, status)
				},
				PlanSchedulableWait,
				fakePlanCommandProvider{
					commandID: "K0sUpdate",
					handlerNewPlan: func(ctx context.Context, pc apv1beta2.PlanCommand, pcs *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
						pcs.K0sUpdate = &apv1beta2.PlanCommandK0sUpdateStatus{}

						return PlanSchedulableWait, false, nil
					},
					handlerSchedulable: func(ctx context.Context, planID string, pc apv1beta2.PlanCommand, pcs *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
						return PlanSchedulableWait, false, fmt.Errorf("should not have reached schedulable")
					},
					handlerSchedulableWait: func(ctx context.Context, planID string, pc apv1beta2.PlanCommand, pcs *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
						return PlanSchedulableWait, false, fmt.Errorf("should not have reached schedulablewait")
					},
				},
			),
			ProviderResultSuccess,
			false,
			&apv1beta2.PlanStatus{
				State: PlanSchedulableWait,
				Commands: []apv1beta2.PlanCommandStatus{
					{
						ID:        0,
						K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{},
					},
					{
						ID:        1,
						K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{},
					},
				},
			},
		},

		// A plan without any commands should get drop to the success state (PlanSchedulableWait)
		{
			"NoPlanCommands",
			&apv1beta2.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "NoPlanCommands",
				},
			},
			NewInitProvidersHandler(
				logger,
				func(ctx context.Context, provider PlanCommandProvider, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
					return provider.Schedulable(ctx, planID, cmd, status)
				},
				PlanSchedulableWait,
				fakePlanCommandProvider{
					commandID: "UnknownProvider",
				},
			),
			ProviderResultSuccess,
			false,
			&apv1beta2.PlanStatus{
				State: PlanSchedulableWait,
			},
		},

		// A scenario where no matching providers are registered
		{
			"UnknownProviders",
			&apv1beta2.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "UnknownProviders",
				},
				Spec: apv1beta2.PlanSpec{
					Commands: []apv1beta2.PlanCommand{
						{
							K0sUpdate: &apv1beta2.PlanCommandK0sUpdate{},
						},
					},
				},
			},
			NewInitProvidersHandler(
				logger,
				func(ctx context.Context, provider PlanCommandProvider, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
					return provider.Schedulable(ctx, planID, cmd, status)
				},
				PlanSchedulableWait,
				fakePlanCommandProvider{
					commandID: "UnknownProvider",
				},
			),
			ProviderResultFailure,
			true,
			nil,
		},

		// A scenario where the adapter returns an error
		{
			"AdapterError",
			&apv1beta2.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "AdapterError",
				},
				Spec: apv1beta2.PlanSpec{
					Commands: []apv1beta2.PlanCommand{
						{
							K0sUpdate: &apv1beta2.PlanCommandK0sUpdate{},
						},
					},
				},
			},
			NewInitProvidersHandler(
				logger,
				func(ctx context.Context, provider PlanCommandProvider, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
					return PlanSchedulableWait, false, fmt.Errorf("intentional error")
				},
				PlanSchedulableWait,
				fakePlanCommandProvider{
					commandID: "K0sUpdate",
				},
			),
			ProviderResultFailure,
			true,
			nil,
		},
	}

	scheme := runtime.NewScheme()
	assert.NoError(t, apscheme2.AddToScheme(scheme))
	assert.NoError(t, v1.AddToScheme(scheme))

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.TODO()
			res, err := test.handler.Handle(ctx, test.plan)
			assert.Equal(t, test.expectedError, err != nil, "Unexpected error: %v", err)
			assert.Equal(t, test.expectedResult, res)

			if test.expectedPlanStatus != nil {
				assert.Equal(t, *test.expectedPlanStatus, test.plan.Status)
			}
		})
	}
}
