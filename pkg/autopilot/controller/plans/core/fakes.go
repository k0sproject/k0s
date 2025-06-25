// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
