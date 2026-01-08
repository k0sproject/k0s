//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/k0sproject/k0s/internal/sync/value"
	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	"github.com/k0sproject/k0s/pkg/autopilot/controller/plans"
	aproot "github.com/k0sproject/k0s/pkg/autopilot/controller/root"
	"github.com/k0sproject/k0s/pkg/autopilot/controller/signal"
	"github.com/k0sproject/k0s/pkg/autopilot/controller/updates"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	cr "sigs.k8s.io/controller-runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crconfig "sigs.k8s.io/controller-runtime/pkg/config"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
	crmetricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	crwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"
)

type leaderElector interface {
	Run(context.Context, func(leaderelection.Status))
}

type subControllerStartRoutineFunc func(ctx context.Context, logger *logrus.Entry, event leaderelection.Status) error
type createLeaderElectorFunc func(leaderelection.Config) (leaderElector, error)

type rootController struct {
	cfg                    aproot.RootConfig
	log                    *logrus.Entry
	kubeClientFactory      kubernetes.ClientFactoryInterface
	autopilotClientFactory apcli.FactoryInterface
	enableWorker           bool
	apiAddress             netip.Addr

	startSubHandlerRoutine subControllerStartRoutineFunc
	newLeaderElector       createLeaderElectorFunc

	initialized bool
}

var _ aproot.Root = (*rootController)(nil)

// NewRootController builds a root for autopilot "controller" operations.
func NewRootController(cfg aproot.RootConfig, logger *logrus.Entry, enableWorker bool, acf apcli.FactoryInterface, apiAddress netip.Addr) (aproot.Root, error) {
	c := &rootController{
		cfg:                    cfg,
		log:                    logger,
		autopilotClientFactory: acf,
		kubeClientFactory:      acf.Unwrap(),
		enableWorker:           enableWorker,
		apiAddress:             apiAddress,
	}

	// Default implementations that can be overridden for testing.
	c.startSubHandlerRoutine = c.startSubControllerRoutine
	c.newLeaderElector = func(c leaderelection.Config) (leaderElector, error) {
		return leaderelection.NewClient(c)
	}

	return c, nil
}

func (c *rootController) Run(ctx context.Context) error {
	// Create / initialize kubernetes objects as needed
	if err := wait.PollUntilContextCancel(ctx, 10*time.Second, true, func(ctx context.Context) (done bool, err error) {
		if err := c.setup(ctx); err != nil {
			c.log.WithError(err).Error("Setup controller failed to complete, retrying in 10 seconds")
			return false, nil
		}
		return true, nil
	}); err != nil {
		return fmt.Errorf("setup controller failed to complete: %w", err)
	}

	kubeClient, err := c.autopilotClientFactory.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes client: %w", err)
	}

	status := value.NewLatest(leaderelection.StatusPending)
	le, err := c.newLeaderElector(&leaderelection.LeaseConfig{
		Namespace: apconst.AutopilotNamespace,
		Name:      apconst.AutopilotNamespace + "-controller",
		Identity:  c.cfg.InvocationID,
		Client:    kubeClient.CoordinationV1(),
	})
	if err != nil {
		return fmt.Errorf("failed to create leader elector: %w", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		le.Run(ctx, status.Set)
	}()

	// Start controllers
	leaseEventStatus, leaseEventStatusExpired := status.Peek()
	subControllerCancel, errCh := c.startSubControllers(ctx, leaseEventStatus)

	for {
		select {
		case <-ctx.Done():
			c.log.Info("Shutting down")
			<-done
			if err := <-errCh; err != nil {
				return fmt.Errorf("while shutting down sub controllers: %w", err)
			}
			return nil

		case <-leaseEventStatusExpired:
			lastLeaseEventStatus := leaseEventStatus
			leaseEventStatus, leaseEventStatusExpired = status.Peek()

			// Don't terminate controllers on receipt of the same lease event.
			if lastLeaseEventStatus == leaseEventStatus {
				c.log.Warnf("Ignoring redundant lease event status (%v == %v)", lastLeaseEventStatus, leaseEventStatus)
				continue
			}

			c.log.Infof("Got lease event = %v, reconfiguring controllers", leaseEventStatus)

			// Stop controllers + wait for termination
			subControllerCancel(fmt.Errorf("lease status changed: %s", leaseEventStatus))
			if err := <-errCh; err != nil {
				c.log.WithError(err).Error("Error while stopping sub controllers")
			}

			// Start controllers
			subControllerCancel, errCh = c.startSubControllers(ctx, leaseEventStatus)
		}
	}
}

// startSubControllerRoutine is what is executed by default by `startSubControllers`.
// This creates the controller-runtime manager, registers all required components,
// and starts it in a goroutine.
func (c *rootController) startSubControllerRoutine(ctx context.Context, logger *logrus.Entry, event leaderelection.Status) error {
	managerOpts := crman.Options{
		Scheme: scheme,
		Controller: crconfig.Controller{
			// If this controller is already initialized, this means that all
			// controller-runtime controllers have already been successfully
			// registered to another manager. However, controller-runtime
			// maintains a global checklist of controller names and doesn't
			// currently provide a way to unregister names from discarded
			// managers. So it's necessary to suppress the global name check
			// whenever things are restarted for reconfiguration.
			SkipNameValidation: ptr.To(c.initialized),
		},
		WebhookServer: crwebhook.NewServer(crwebhook.Options{
			Port: c.cfg.ManagerPort,
		}),
		Metrics: crmetricsserver.Options{
			BindAddress: c.cfg.MetricsBindAddr,
		},
		HealthProbeBindAddress: c.cfg.HealthProbeBindAddr,
	}

	restConfig, err := c.autopilotClientFactory.GetRESTConfig()
	if err != nil {
		return err
	}

	mgr, err := cr.NewManager(restConfig, managerOpts)
	if err != nil {
		logger.WithError(err).Error("unable to start controller manager")
		return err
	}

	if err := RegisterIndexers(ctx, mgr, "controller"); err != nil {
		logger.WithError(err).Error("unable to register indexers")
		return err
	}

	leaderMode := event == leaderelection.StatusLeading

	k0sClient, err := c.kubeClientFactory.GetK0sClient()
	if err != nil {
		return fmt.Errorf("failed to get k0s client: %w", err)
	}
	tlsConfig, err := rest.TLSConfigFor(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes TLS config: %w", err)
	}
	prober := newReadyProber(logger, k0sClient, tlsConfig, c.cfg.KubeAPIPort, 1*time.Minute)

	delegateMap := map[string]apdel.ControllerDelegate{
		apdel.ControllerDelegateWorker: apdel.NodeControllerDelegate(),
		apdel.ControllerDelegateController: apdel.ControlNodeControllerDelegate(apdel.WithReadyForUpdateFunc(
			func(ctx context.Context, status apv1beta2.PlanCommandK0sUpdateStatus, obj crcli.Object) apdel.K0sUpdateReadyStatus {
				if err := prober.probeTargets(ctx, status.Controllers); err != nil {
					logger.WithError(err).Error("Plan can not be applied to controllers (failed unanimous)")
					return apdel.Inconsistent
				}

				return apdel.CanUpdate
			},
		)),
	}

	cl, err := c.autopilotClientFactory.GetClient()
	if err != nil {
		return err
	}
	ns, err := cl.CoreV1().Namespaces().Get(ctx, metav1.NamespaceSystem, metav1.GetOptions{})
	if err != nil {
		return err
	}
	clusterID := string(ns.UID)

	if err := signal.RegisterControllers(ctx, logger, mgr, delegateMap[apdel.ControllerDelegateController], c.cfg.K0sDataDir, clusterID); err != nil {
		logger.WithError(err).Error("unable to register signal controllers")
		return err
	}

	if err := plans.RegisterControllers(ctx, logger, mgr, c.kubeClientFactory, leaderMode, delegateMap, c.cfg.ExcludeFromPlans); err != nil {
		logger.WithError(err).Error("unable to register plans controllers")
		return err
	}

	if err := updates.RegisterControllers(ctx, logger, mgr, c.autopilotClientFactory, leaderMode, clusterID); err != nil {
		logger.WithError(err).Error("unable to register updates controllers")
		return err
	}

	// All the controller-runtime controllers have been registered.
	c.initialized = true

	// The controller-runtime start blocks until the context is canceled.
	if err := mgr.Start(ctx); err != nil {
		logger.WithError(err).Error("unable to run controller-runtime manager")
		return err
	}

	return nil
}

// startSubControllers starts all of the controllers specific to the leader mode.
func (c *rootController) startSubControllers(ctx context.Context, event leaderelection.Status) (context.CancelCauseFunc, <-chan error) {
	logger := c.log.WithField("leadermode", event == leaderelection.StatusLeading)
	logger.Info("Starting subcontrollers")

	ctx, cancel := context.WithCancelCause(ctx)
	errCh := make(chan error, 1)

	go func() {
		var err error
		defer func() { close(errCh); cancel(err) }()
		err = c.startSubHandlerRoutine(ctx, logger, event)
		errCh <- err
	}()

	return cancel, errCh
}
