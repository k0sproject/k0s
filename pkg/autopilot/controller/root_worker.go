//go:build unix

// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"time"

	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	aproot "github.com/k0sproject/k0s/pkg/autopilot/controller/root"
	"github.com/k0sproject/k0s/pkg/autopilot/controller/signal"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	k8sretry "k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	cr "sigs.k8s.io/controller-runtime"
	crconfig "sigs.k8s.io/controller-runtime/pkg/config"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
	crmetricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	crwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/sirupsen/logrus"
)

type rootWorker struct {
	cfg           aproot.RootConfig
	log           *logrus.Entry
	clientFactory apcli.FactoryInterface

	initialized bool
}

var _ aproot.Root = (*rootWorker)(nil)

// NewRootWorker builds a root for autopilot "worker" operations.
func NewRootWorker(cfg aproot.RootConfig, logger *logrus.Entry, cf apcli.FactoryInterface) (aproot.Root, error) {
	c := &rootWorker{
		cfg:           cfg,
		log:           logger,
		clientFactory: cf,
	}

	return c, nil
}

func (w *rootWorker) Run(ctx context.Context) error {
	logger := w.log

	managerOpts := crman.Options{
		Scheme: scheme,
		Controller: crconfig.Controller{
			// If this controller is already initialized, this means that all
			// controller-runtime controllers have already been successfully
			// registered to another manager. However, controller-runtime
			// maintains a global checklist of controller names and doesn't
			// currently provide a way to unregister names from discarded
			// managers. So it's necessary to suppress the global name check
			// whenever things retried.
			SkipNameValidation: ptr.To(w.initialized),
		},
		WebhookServer: crwebhook.NewServer(crwebhook.Options{
			Port: w.cfg.ManagerPort,
		}),
		Metrics: crmetricsserver.Options{
			BindAddress: w.cfg.MetricsBindAddr,
		},
		HealthProbeBindAddress: w.cfg.HealthProbeBindAddr,
	}

	// In some cases, we need to wait on the worker side until controller deploys all autopilot CRDs
	var attempt uint
	return k8sretry.OnError(wait.Backoff{
		Steps:    120,
		Duration: 1 * time.Second,
		Factor:   1.0,
		Jitter:   0.1,
	}, func(err error) bool {
		attempt++
		logger := logger.WithError(err).WithField("attempt", attempt)
		logger.Debug("Failed to run controller manager, retrying after backoff")
		return true
	}, func() error {
		cl, err := w.clientFactory.GetClient()
		if err != nil {
			return err
		}
		ns, err := cl.CoreV1().Namespaces().Get(ctx, metav1.NamespaceSystem, metav1.GetOptions{})
		if err != nil {
			return err
		}
		clusterID := string(ns.UID)

		restConfig, err := w.clientFactory.GetRESTConfig()
		if err != nil {
			return err
		}

		mgr, err := cr.NewManager(restConfig, managerOpts)
		if err != nil {
			return fmt.Errorf("failed to create controller manager: %w", err)
		}

		if err := RegisterIndexers(ctx, mgr, "worker"); err != nil {
			return fmt.Errorf("unable to register indexers: %w", err)
		}

		if err := signal.RegisterControllers(ctx, logger, mgr, apdel.NodeControllerDelegate(), w.cfg.K0sDataDir, true, clusterID, leaderelection.StatusPending, w.cfg.InvocationID); err != nil {
			return fmt.Errorf("unable to register signal controllers: %w", err)
		}

		// All the controller-runtime controllers have been registered.
		w.initialized = true

		// The controller-runtime start blocks until the context is canceled.
		if err := mgr.Start(ctx); err != nil {
			return fmt.Errorf("unable to run controller-runtime manager for workers: %w", err)
		}
		return nil
	})
}
