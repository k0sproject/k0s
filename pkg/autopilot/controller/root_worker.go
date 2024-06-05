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

	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	aproot "github.com/k0sproject/k0s/pkg/autopilot/controller/root"
	apsig "github.com/k0sproject/k0s/pkg/autopilot/controller/signal"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	k8sretry "k8s.io/client-go/util/retry"
	cr "sigs.k8s.io/controller-runtime"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
	crmetricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	crwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/avast/retry-go"
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

	var mgr crman.Manager
	if err := retry.Do(
		func() (err error) {
			mgr, err = cr.NewManager(w.clientFactory.RESTConfig(), managerOpts)
			return err
		},
		retry.Context(ctx),
		retry.LastErrorOnly(true),
		retry.Delay(1*time.Second),
		retry.OnRetry(func(attempt uint, err error) {
			logger.WithError(err).Debugf("Failed to start controller manager in attempt #%d, retrying after backoff", attempt+1)
		}),
	); err != nil {
		logger.WithError(err).Fatal("unable to start controller manager")
	}

	// In some cases, we need to wait on the worker side until controller deploys all autopilot CRDs
	return k8sretry.OnError(wait.Backoff{
		Steps:    120,
		Duration: 1 * time.Second,
		Factor:   1.0,
		Jitter:   0.1,
	}, func(err error) bool {
		return true
	}, func() error {
		cl, err := w.clientFactory.GetClient()
		if err != nil {
			return err
		}
		ns, err := cl.CoreV1().Namespaces().Get(ctx, "kube-system", v1.GetOptions{})
		if err != nil {
			return err
		}
		clusterID := string(ns.UID)

		if err := RegisterIndexers(ctx, mgr, "worker"); err != nil {
			return fmt.Errorf("unable to register indexers: %w", err)
		}

		if err := apsig.RegisterControllers(ctx, logger, mgr, apdel.NodeControllerDelegate(), w.cfg.K0sDataDir, clusterID); err != nil {
			return fmt.Errorf("unable to register 'controlnodes' controllers: %w", err)
		}
		// The controller-runtime start blocks until the context is cancelled.
		if err := mgr.Start(ctx); err != nil {
			return fmt.Errorf("unable to run controller-runtime manager for workers: %w", err)
		}
		return nil
	})
}
