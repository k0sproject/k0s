//go:build unix

// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	aproot "github.com/k0sproject/k0s/pkg/autopilot/controller/root"
	"github.com/k0sproject/k0s/pkg/autopilot/controller/signal"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cr "sigs.k8s.io/controller-runtime"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
	crmetricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	crwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/sirupsen/logrus"
)

type rootWorker struct {
	cfg           aproot.RootConfig
	log           *logrus.Entry
	clientFactory apcli.FactoryInterface
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
		WebhookServer: crwebhook.NewServer(crwebhook.Options{
			Port: w.cfg.ManagerPort,
		}),
		Metrics: crmetricsserver.Options{
			BindAddress: w.cfg.MetricsBindAddr,
		},
		HealthProbeBindAddress: w.cfg.HealthProbeBindAddr,
	}

	clusterID, err := w.getClusterID(ctx)
	if err != nil {
		return err
	}

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

	if err := signal.RegisterControllers(ctx, logger, mgr, apdel.NodeControllerDelegate(), w.cfg.K0sDataDir, clusterID); err != nil {
		return fmt.Errorf("unable to register signal controllers: %w", err)
	}

	// The controller-runtime start blocks until the context is canceled.
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("unable to run controller-runtime manager for workers: %w", err)
	}
	return nil
}

func (w *rootWorker) getClusterID(ctx context.Context) (string, error) {
	client, err := w.clientFactory.GetClient()
	if err != nil {
		return "", err
	}

	namespace, err := client.CoreV1().Namespaces().Get(ctx, metav1.NamespaceSystem, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return string(namespace.UID), nil
}
