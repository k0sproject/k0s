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

package controller

import (
	"context"
	"fmt"
	"time"

	apv1beta2 "github.com/k0sproject/k0s/pkg/autopilot/apis/autopilot.k0sproject.io/v1beta2"
	apscheme "github.com/k0sproject/k0s/pkg/autopilot/apis/autopilot.k0sproject.io/v1beta2/clientset/scheme"
	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	applan "github.com/k0sproject/k0s/pkg/autopilot/controller/plans"
	aproot "github.com/k0sproject/k0s/pkg/autopilot/controller/root"
	apsig "github.com/k0sproject/k0s/pkg/autopilot/controller/signal"
	apupdate "github.com/k0sproject/k0s/pkg/autopilot/controller/updates"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	cr "sigs.k8s.io/controller-runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
)

type subControllerStartFunc func(ctx context.Context, event LeaseEventStatus) (context.CancelFunc, *errgroup.Group)
type subControllerStartRoutineFunc func(ctx context.Context, logger *logrus.Entry, event LeaseEventStatus) error
type subControllerStopFunc func(cancel context.CancelFunc, g *errgroup.Group, event LeaseEventStatus)
type leaseWatcherCreatorFunc func(*logrus.Entry, apcli.FactoryInterface) (LeaseWatcher, error)
type setupFunc func(ctx context.Context, cf apcli.FactoryInterface) error

type rootController struct {
	cfg           aproot.RootConfig
	log           *logrus.Entry
	clientFactory apcli.FactoryInterface

	startSubHandler        subControllerStartFunc
	startSubHandlerRoutine subControllerStartRoutineFunc
	stopSubHandler         subControllerStopFunc
	leaseWatcherCreator    leaseWatcherCreatorFunc
	setupHandler           setupFunc
}

var _ aproot.Root = (*rootController)(nil)

// NewRootController builds a root for autopilot "controller" operations.
func NewRootController(cfg aproot.RootConfig, logger *logrus.Entry, cf apcli.FactoryInterface) (aproot.Root, error) {
	c := &rootController{
		cfg:           cfg,
		log:           logger,
		clientFactory: cf,
	}

	// Default implementations that can be overridden for testing.
	c.startSubHandler = c.startSubControllers
	c.startSubHandlerRoutine = c.startSubControllerRoutine
	c.stopSubHandler = c.stopSubControllers
	c.leaseWatcherCreator = NewLeaseWatcher
	c.setupHandler = func(ctx context.Context, cf apcli.FactoryInterface) error {
		setupController := NewSetupController(c.log, cf, cfg.K0sDataDir)
		return setupController.Run(ctx)
	}

	return c, nil
}

func (c *rootController) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	_ = cancel

	// Create / initialize kubernetes objects as needed
	if err := c.setupHandler(ctx, c.clientFactory); err != nil {
		return fmt.Errorf("setup controller failed to complete: %w", err)
	}

	leaseWatcher, err := c.leaseWatcherCreator(c.log, c.clientFactory)
	if err != nil {
		return fmt.Errorf("unable to setup lease watcher: %w", err)
	}

	leaseEventStatusCh, errorCh := leaseWatcher.StartWatcher(ctx, apconst.AutopilotNamespace, fmt.Sprintf("%s-controller", apconst.AutopilotNamespace))

	var lastLeaseEventStatus LeaseEventStatus
	var subControllerCancel context.CancelFunc
	var subControllerErrGroup *errgroup.Group

	for {
		select {
		case err := <-errorCh:
			return err

		case <-ctx.Done():
			c.log.Info("Shutting down")
			c.stopSubHandler(subControllerCancel, subControllerErrGroup, LeaseAcquired)

			return nil

		case leaseEventStatus, ok := <-leaseEventStatusCh:
			if !ok {
				c.log.Warn("lease event status channel closed")
				return nil
			}

			// Don't terminate controllers on receipt of the same lease event.
			if lastLeaseEventStatus == leaseEventStatus {
				c.log.Warnf("Ignoring redundant lease event status (%v == %v)", lastLeaseEventStatus, leaseEventStatus)
				continue
			}

			c.log.Infof("Got lease event = %v, reconfiguring controllers", leaseEventStatus)

			// Stop controllers + wait for termination
			c.stopSubHandler(subControllerCancel, subControllerErrGroup, leaseEventStatus)

			// Start controllers
			subControllerCancel, subControllerErrGroup = c.startSubHandler(ctx, leaseEventStatus)

			// Remember which mode we're in
			lastLeaseEventStatus = leaseEventStatus
		}
	}
}

// startSubControllerRoutine is what is executed by default by `startSubControllers`.
// This creates the controller-runtime manager, registers all required components,
// and starts it in a goroutine.
func (c *rootController) startSubControllerRoutine(ctx context.Context, logger *logrus.Entry, event LeaseEventStatus) error {
	managerOpts := crman.Options{
		Port:                   c.cfg.ManagerPort,
		MetricsBindAddress:     c.cfg.MetricsBindAddr,
		HealthProbeBindAddress: c.cfg.HealthProbeBindAddr,
	}

	mgr, err := cr.NewManager(c.clientFactory.RESTConfig(), managerOpts)
	if err != nil {
		logger.WithError(err).Error("unable to start controller manager")
		return err
	}

	if err := apscheme.AddToScheme(mgr.GetScheme()); err != nil {
		logger.WithError(err).Error("unable to register autopilot scheme")
		return err
	}

	if err := RegisterIndexers(ctx, mgr, "controller"); err != nil {
		logger.WithError(err).Error("unable to register indexers")
		return err
	}

	leaderMode := event == LeaseAcquired

	prober, err := NewReadyProber(logger, c.clientFactory, mgr.GetConfig(), 1*time.Minute)
	if err != nil {
		logger.WithError(err).Error("unable to create controller prober: %w")
		return err
	}

	delegateMap := map[string]apdel.ControllerDelegate{
		apdel.ControllerDelegateWorker: apdel.NodeControllerDelegate(),
		apdel.ControllerDelegateController: apdel.ControlNodeControllerDelegate(apdel.WithReadyForUpdateFunc(
			func(status apv1beta2.PlanCommandK0sUpdateStatus, obj crcli.Object) apdel.K0sUpdateReadyStatus {
				prober.AddTargets(status.Controllers)

				if err := prober.Probe(); err != nil {
					logger.WithError(err).Error("Plan can not be applied to controllers (failed unanimous)")
					return apdel.Inconsistent
				}

				return apdel.CanUpdate
			},
		)),
	}

	if err := apsig.RegisterControllers(ctx, logger, mgr, delegateMap[apdel.ControllerDelegateController], c.cfg.K0sDataDir); err != nil {
		logger.WithError(err).Error("unable to register 'signal' controllers")
		return err
	}

	if err := applan.RegisterControllers(ctx, logger, mgr, leaderMode, delegateMap, c.cfg.ExcludeFromPlans); err != nil {
		logger.WithError(err).Error("unable to register 'plans' controllers")
		return err
	}

	if err := apupdate.RegisterControllers(ctx, logger, mgr, c.clientFactory, leaderMode); err != nil {
		logger.WithError(err).Error("unable to register 'update' controllers")
		return err
	}

	// The controller-runtime start blocks until the context is cancelled.
	if err := mgr.Start(ctx); err != nil {
		logger.WithError(err).Error("unable to run controller-runtime manager")
		return err
	}

	return nil
}

// startSubControllers starts all of the controllers specific to the leader mode.
// It is expected that this function runs to completion.
func (c *rootController) startSubControllers(ctx context.Context, event LeaseEventStatus) (context.CancelFunc, *errgroup.Group) {
	logger := c.log.WithField("leadermode", event == LeaseAcquired)
	logger.Info("Starting subcontrollers")

	ctx, cancel := context.WithCancel(ctx)

	//wg := sync.WaitGroup{}
	g, ctx := errgroup.WithContext(ctx)
	//wg.Add(1)

	g.Go(func() error {
		logger.Info("Starting controller-runtime subhandlers")
		if err := c.startSubHandlerRoutine(ctx, logger, event); err != nil {
			return fmt.Errorf("failed to start subhandlers: %w", err)
		}
		return nil
	})

	return cancel, g
}

// startSubControllers stop all of the controllers specific to the leader mode.
func (c *rootController) stopSubControllers(cancel context.CancelFunc, g *errgroup.Group, event LeaseEventStatus) {
	logger := c.log.WithField("leasemode", event)
	logger.Info("Stopping subcontrollers")

	if cancel != nil {
		cancel()
		if err := g.Wait(); err != nil {
			logger.Error(err)
		}
	}
}
