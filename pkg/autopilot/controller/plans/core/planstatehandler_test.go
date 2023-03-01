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

// TestHandle runs through a table of scenarios focusing on the edge cases of the `Handle()` function
// in `PlanStateHandler`
func TestHandle(t *testing.T) {
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
					},
				},
				Status: apv1beta2.PlanStatus{
					State: PlanSchedulable,
				},
			},
			NewPlanStateHandler(
				logger,
				func(ctx context.Context, provider PlanCommandProvider, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
					return provider.Schedulable(ctx, planID, cmd, status)
				},
				fakePlanCommandProvider{
					commandID: "K0sUpdate",
					handlerNewPlan: func(ctx context.Context, pc apv1beta2.PlanCommand, pcs *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
						return PlanSchedulableWait, false, fmt.Errorf("should not have reached newplan")
					},
					handlerSchedulable: func(ctx context.Context, planID string, pc apv1beta2.PlanCommand, pcs *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
						assert.Equal(t, "v1.2.3", pc.K0sUpdate.Version)
						pcs.K0sUpdate = &apv1beta2.PlanCommandK0sUpdateStatus{}

						return PlanCompleted, false, nil
					},
					handlerSchedulableWait: func(ctx context.Context, planID string, pc apv1beta2.PlanCommand, pcs *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
						return PlanSchedulableWait, false, fmt.Errorf("should not have reached schedulablewait")
					},
				},
			),
			ProviderResultSuccess,
			false,
			&apv1beta2.PlanStatus{
				State: PlanSchedulable,
				Commands: []apv1beta2.PlanCommandStatus{
					{
						State:     PlanCompleted,
						K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{},
					},
				},
			},
		},

		// A plan without any commands should get 'Completed' successfully
		{
			"NoPlanCommands",
			&apv1beta2.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "NoPlanCommands",
				},
			},
			NewPlanStateHandler(
				logger,
				func(ctx context.Context, provider PlanCommandProvider, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
					return provider.Schedulable(ctx, planID, cmd, status)
				},
				fakePlanCommandProvider{
					commandID: "UnknownProvider",
				},
			),
			ProviderResultSuccess,
			false,
			&apv1beta2.PlanStatus{
				State: PlanCompleted,
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
			NewPlanStateHandler(
				logger,
				func(ctx context.Context, provider PlanCommandProvider, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
					return provider.Schedulable(ctx, planID, cmd, status)
				},
				fakePlanCommandProvider{
					commandID: "UnknownProvider",
				},
			),
			ProviderResultFailure,
			true,
			nil,
		},

		// The scenario where the number of command statuses don't match the number of commands
		{
			"BrokenStatusSymmetry",
			&apv1beta2.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "BrokenStatusSymmetry",
				},
				Spec: apv1beta2.PlanSpec{
					Commands: []apv1beta2.PlanCommand{
						{
							K0sUpdate: &apv1beta2.PlanCommandK0sUpdate{},
						},
					},
				},
				Status: apv1beta2.PlanStatus{
					Commands: []apv1beta2.PlanCommandStatus{
						{
							State:     PlanCompleted,
							K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{},
						},
						{
							State:     PlanCompleted,
							K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{},
						},
					},
				},
			},
			NewPlanStateHandler(
				logger,
				nil,
				fakePlanCommandProvider{
					commandID: "K0sUpdate",
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
			NewPlanStateHandler(
				logger,
				func(ctx context.Context, provider PlanCommandProvider, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
					return PlanSchedulableWait, false, fmt.Errorf("intentional error")
				},
				fakePlanCommandProvider{
					commandID: "K0sUpdate",
				},
			),
			ProviderResultFailure,
			true,
			nil,
		},

		// Ensures that if a command is marked as 'Completed' it is skipped in favor for the next one.
		{
			"SkipCompletedCommand",
			&apv1beta2.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "SkipCompletedCommand",
				},
				Spec: apv1beta2.PlanSpec{
					Commands: []apv1beta2.PlanCommand{
						{
							K0sUpdate: &apv1beta2.PlanCommandK0sUpdate{
								Version: "v1",
							},
						},
						{
							K0sUpdate: &apv1beta2.PlanCommandK0sUpdate{
								Version: "v2",
							},
						},
					},
				},
				Status: apv1beta2.PlanStatus{
					Commands: []apv1beta2.PlanCommandStatus{
						{
							State:     PlanCompleted,
							K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{},
						},
						{
							State:     PlanSchedulable,
							K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{},
						},
					},
				},
			},
			NewPlanStateHandler(
				logger,
				func(ctx context.Context, provider PlanCommandProvider, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
					return provider.Schedulable(ctx, planID, cmd, status)
				},
				fakePlanCommandProvider{
					commandID: "K0sUpdate",
					handlerNewPlan: func(ctx context.Context, pc apv1beta2.PlanCommand, pcs *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
						return pcs.State, false, fmt.Errorf("should not have reached newplan")
					},
					handlerSchedulable: func(ctx context.Context, planID string, pc apv1beta2.PlanCommand, pcs *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
						// Ensures that only the second command makes it here
						assert.Equal(t, "v2", pc.K0sUpdate.Version)

						pcs.K0sUpdate = &apv1beta2.PlanCommandK0sUpdateStatus{}

						return PlanCompleted, false, nil
					},
					handlerSchedulableWait: func(ctx context.Context, planID string, pc apv1beta2.PlanCommand, pcs *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
						return pcs.State, false, fmt.Errorf("should not have reached schedulablewait")
					},
				},
			),
			ProviderResultSuccess,
			false,
			&apv1beta2.PlanStatus{
				// Note: the plan state is unchanged as that needs another iteration.
				Commands: []apv1beta2.PlanCommandStatus{
					{
						State:     PlanCompleted,
						K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{},
					},
					{
						State:     PlanCompleted,
						K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{},
					},
				},
			},
		},

		// If no commands are present, ensure the status moves to 'Completed'
		{
			"MarkCompletedEmpty",
			&apv1beta2.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "MarkCompletedEmpty",
				},
				Spec: apv1beta2.PlanSpec{
					Commands: []apv1beta2.PlanCommand{},
				},
				Status: apv1beta2.PlanStatus{
					State: PlanSchedulable,
				},
			},
			NewPlanStateHandler(
				logger,
				func(ctx context.Context, provider PlanCommandProvider, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
					return provider.Schedulable(ctx, planID, cmd, status)
				},
				fakePlanCommandProvider{
					commandID: "K0sUpdate",
					handlerNewPlan: func(ctx context.Context, pc apv1beta2.PlanCommand, pcs *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
						return pcs.State, false, fmt.Errorf("should not have reached newplan")
					},
					handlerSchedulable: func(ctx context.Context, planID string, pc apv1beta2.PlanCommand, pcs *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
						pcs.K0sUpdate = &apv1beta2.PlanCommandK0sUpdateStatus{}
						return PlanCompleted, false, nil
					},
					handlerSchedulableWait: func(ctx context.Context, planID string, pc apv1beta2.PlanCommand, pcs *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
						return pcs.State, false, fmt.Errorf("should not have reached schedulablewait")
					},
				},
			),
			ProviderResultSuccess,
			false,
			&apv1beta2.PlanStatus{
				State: PlanCompleted,
			},
		},

		// If all of the commands available are marked as 'Completed', then the plan needs to
		// move to 'Completed'.
		{
			"MarkCompleted",
			&apv1beta2.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "MarkCompleted",
				},
				Spec: apv1beta2.PlanSpec{
					Commands: []apv1beta2.PlanCommand{
						{
							K0sUpdate: &apv1beta2.PlanCommandK0sUpdate{},
						},
					},
				},
				Status: apv1beta2.PlanStatus{
					State: PlanSchedulable,
					Commands: []apv1beta2.PlanCommandStatus{
						{
							State:     PlanCompleted,
							K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{},
						},
					},
				},
			},
			NewPlanStateHandler(
				logger,
				func(ctx context.Context, provider PlanCommandProvider, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
					return provider.Schedulable(ctx, planID, cmd, status)
				},
				fakePlanCommandProvider{
					commandID: "K0sUpdate",
					handlerNewPlan: func(ctx context.Context, pc apv1beta2.PlanCommand, pcs *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
						return pcs.State, false, fmt.Errorf("should not have reached newplan")
					},
					handlerSchedulable: func(ctx context.Context, planID string, pc apv1beta2.PlanCommand, pcs *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
						pcs.K0sUpdate = &apv1beta2.PlanCommandK0sUpdateStatus{}
						return PlanCompleted, false, nil
					},
					handlerSchedulableWait: func(ctx context.Context, planID string, pc apv1beta2.PlanCommand, pcs *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
						return pcs.State, false, fmt.Errorf("should not have reached schedulablewait")
					},
				},
			),
			ProviderResultSuccess,
			false,
			&apv1beta2.PlanStatus{
				State: PlanCompleted,
				Commands: []apv1beta2.PlanCommandStatus{
					{
						State:     PlanCompleted,
						K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{},
					},
				},
			},
		},

		// Ensure that retry is returned when the adapter requests it
		{
			"Retry",
			&apv1beta2.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "Retry",
				},
				Spec: apv1beta2.PlanSpec{
					Commands: []apv1beta2.PlanCommand{
						{
							K0sUpdate: &apv1beta2.PlanCommandK0sUpdate{},
						},
					},
				},
				Status: apv1beta2.PlanStatus{
					State: PlanSchedulable,
					Commands: []apv1beta2.PlanCommandStatus{
						{
							State:     PlanSchedulable,
							K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{},
						},
					},
				},
			},
			NewPlanStateHandler(
				logger,
				func(ctx context.Context, provider PlanCommandProvider, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
					return provider.Schedulable(ctx, planID, cmd, status)
				},
				fakePlanCommandProvider{
					commandID: "K0sUpdate",
					handlerNewPlan: func(ctx context.Context, pc apv1beta2.PlanCommand, pcs *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
						return pcs.State, false, fmt.Errorf("should not have reached newplan")
					},
					handlerSchedulable: func(ctx context.Context, planID string, pc apv1beta2.PlanCommand, pcs *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
						pcs.K0sUpdate = &apv1beta2.PlanCommandK0sUpdateStatus{}
						return pcs.State, true, nil
					},
					handlerSchedulableWait: func(ctx context.Context, planID string, pc apv1beta2.PlanCommand, pcs *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
						return pcs.State, false, fmt.Errorf("should not have reached schedulablewait")
					},
				},
			),
			ProviderResultRetry,
			false,
			&apv1beta2.PlanStatus{
				State: PlanSchedulable,
				Commands: []apv1beta2.PlanCommandStatus{
					{
						State:     PlanSchedulable,
						K0sUpdate: &apv1beta2.PlanCommandK0sUpdateStatus{},
					},
				},
			},
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
