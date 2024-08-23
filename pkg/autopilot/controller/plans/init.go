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

package plans

import (
	"context"
	"fmt"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	appagupdate "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/cmdprovider/airgapupdate"
	appk0supdate "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/cmdprovider/k0supdate"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	"github.com/k0sproject/k0s/pkg/kubernetes"

	"github.com/sirupsen/logrus"
	cr "sigs.k8s.io/controller-runtime"
	crev "sigs.k8s.io/controller-runtime/pkg/event"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
	crpred "sigs.k8s.io/controller-runtime/pkg/predicate"
)

// RegisterControllers registers all of the autopilot controllers used by `plans`
// to the controller-runtime manager when running in 'controller' mode.
func RegisterControllers(ctx context.Context, logger *logrus.Entry, mgr crman.Manager, cf kubernetes.ClientFactoryInterface, leaderMode bool, controllerDelegateMap apdel.ControllerDelegateMap, excludeFromPlans []string) error {
	logger = logger.WithField("controller", "plans")

	cmdProviders := []appc.PlanCommandProvider{
		appk0supdate.NewK0sUpdatePlanCommandProvider(logger, mgr.GetClient(), controllerDelegateMap, cf, excludeFromPlans),
		appagupdate.NewAirgapUpdatePlanCommandProvider(logger, mgr.GetClient(), controllerDelegateMap, cf, excludeFromPlans),
	}

	if leaderMode {
		if err := registerNewPlanStateController(logger, mgr, cmdProviders); err != nil {
			return fmt.Errorf("unable to register 'newplan' controller: %w", err)
		}

		if err := registerSchedulableWaitStateController(logger, mgr, cmdProviders); err != nil {
			return fmt.Errorf("unable to register 'schedulablewait' controller: %w", err)
		}

		if err := registerSchedulableStateController(logger, mgr, cmdProviders); err != nil {
			return fmt.Errorf("unable to register 'schedulable' controller: %w", err)
		}
	}

	return nil
}

// registerNewPlanStateController registers the 'newplan' plan state controller to
// controller-runtime.
func registerNewPlanStateController(logger *logrus.Entry, mgr crman.Manager, providers []appc.PlanCommandProvider) error {
	handler := appc.NewInitProvidersHandler(
		logger,
		func(ctx context.Context, provider appc.PlanCommandProvider, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
			return provider.NewPlan(ctx, cmd, status)
		},
		appc.PlanSchedulableWait,
		providers...,
	)

	return registerPlanStateController("newplan", logger, mgr, newPlanEventFilter(), handler, providers)
}

// registerSchedulableWaitStateController registers the 'schedulablewait' plan state controller to
// controller-runtime.
func registerSchedulableWaitStateController(logger *logrus.Entry, mgr crman.Manager, providers []appc.PlanCommandProvider) error {
	handler := appc.NewPlanStateHandler(
		logger,
		func(ctx context.Context, provider appc.PlanCommandProvider, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
			return provider.SchedulableWait(ctx, planID, cmd, status)
		},
		providers...,
	)

	return registerPlanStateController("schedulablewait", logger, mgr, schedulableWaitEventFilter(), handler, providers)
}

// registerSchedulableStateController registers the 'schedulable' plan state controller to
// controller-runtime.
func registerSchedulableStateController(logger *logrus.Entry, mgr crman.Manager, providers []appc.PlanCommandProvider) error {
	handler := appc.NewPlanStateHandler(
		logger,
		func(ctx context.Context, provider appc.PlanCommandProvider, planID string, cmd apv1beta2.PlanCommand, status *apv1beta2.PlanCommandStatus) (apv1beta2.PlanStateType, bool, error) {
			return provider.Schedulable(ctx, planID, cmd, status)
		},
		providers...,
	)

	return registerPlanStateController("schedulable", logger, mgr, schedulableEventFilter(), handler, providers)
}

// registerPlanStateController is a helper for registering a plan state controller into
// controller-runtime.
func registerPlanStateController(name string, logger *logrus.Entry, mgr crman.Manager, eventFilter crpred.Predicate, handler appc.PlanStateHandler, providers []appc.PlanCommandProvider) error {
	return cr.NewControllerManagedBy(mgr).
		Named("planstate-" + name).
		For(&apv1beta2.Plan{}).
		WithEventFilter(eventFilter).
		Complete(
			appc.NewPlanStateController(name, logger, mgr.GetClient(), handler),
		)
}

// newPlanEventFilter creates a controller-runtime predicate that governs which
// objects will make it into reconciliation, and which will be ignored.
func newPlanEventFilter() crpred.Predicate {
	return crpred.And(
		PlanNamePredicate(apconst.AutopilotName),
		apcomm.FalseFuncs{
			CreateFunc: func(ce crev.CreateEvent) bool {
				plan, ok := ce.Object.(*apv1beta2.Plan)
				return ok && len(plan.Status.State) == 0
			},
		},
	)
}

// schedulableWaitEventFilter creates a controller-runtime predicate that governs which
// objects will make it into reconciliation, and which will be ignored.
func schedulableWaitEventFilter() crpred.Predicate {
	return crpred.And(
		PlanNamePredicate(apconst.AutopilotName),
		PlanStatusPredicate(appc.PlanSchedulableWait),
		apcomm.FalseFuncs{
			CreateFunc: func(ce crev.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(ue crev.UpdateEvent) bool {
				return true
			},
		},
	)
}

// schedulableEventFilter creates a controller-runtime predicate that governs which
// objects will make it into reconciliation, and which will be ignored.
func schedulableEventFilter() crpred.Predicate {
	return crpred.And(
		PlanNamePredicate(apconst.AutopilotName),
		PlanStatusPredicate(appc.PlanSchedulable),
		apcomm.FalseFuncs{
			CreateFunc: func(ce crev.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(ue crev.UpdateEvent) bool {
				return true
			},
		},
	)
}
