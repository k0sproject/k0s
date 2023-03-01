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

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
)

// fakePlanCommandProvider is a testable `PlanCommandProvider` that provides functions for
// all of the interface methods, allowing for easy testing.
type fakePlanCommandProvider struct {
	commandID              string
	handlerNewPlan         func(context.Context, apv1beta2.PlanCommand, *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error)
	handlerSchedulable     func(context.Context, string, apv1beta2.PlanCommand, *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error)
	handlerSchedulableWait func(context.Context, string, apv1beta2.PlanCommand, *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error)
}

var _ PlanCommandProvider = (*fakePlanCommandProvider)(nil)

func (f fakePlanCommandProvider) CommandID() string {
	return f.commandID
}

func (f fakePlanCommandProvider) NewPlan(ctx context.Context, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
	return f.handlerNewPlan(ctx, cmd, status)
}

func (f fakePlanCommandProvider) Schedulable(ctx context.Context, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
	return f.handlerSchedulable(ctx, planID, cmd, status)
}

func (f fakePlanCommandProvider) SchedulableWait(ctx context.Context, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
	return f.handlerSchedulableWait(ctx, planID, cmd, status)
}
