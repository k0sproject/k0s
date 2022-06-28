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
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"time"

	apscheme "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2/clientset/scheme"
	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	aproot "github.com/k0sproject/k0s/pkg/autopilot/controller/root"
	apsig "github.com/k0sproject/k0s/pkg/autopilot/controller/signal"

	cr "sigs.k8s.io/controller-runtime"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"

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
		Port:                   w.cfg.ManagerPort,
		MetricsBindAddress:     w.cfg.MetricsBindAddr,
		HealthProbeBindAddress: w.cfg.HealthProbeBindAddr,
	}

	mgr, err := cr.NewManager(w.clientFactory.RESTConfig(), managerOpts)
	if err != nil {
		logger.WithError(err).Fatal("unable to start controller manager")
	}

	if err := apscheme.AddToScheme(mgr.GetScheme()); err != nil {
		logger.WithError(err).Fatal("unable to register autopilot scheme")
	}

	// In some cases, we need to wait on the worker side until controller deploys all autopilot CRDs
	return retry.OnError(wait.Backoff{
		Steps:    120,
		Duration: 1 * time.Second,
		Factor:   1.0,
		Jitter:   0.1,
	}, func(err error) bool {
		return true
	}, func() error {
		if err := RegisterIndexers(ctx, mgr, "worker"); err != nil {
			return fmt.Errorf("unable to register indexers: %w", err)
		}

		if err := apsig.RegisterControllers(ctx, logger, mgr, apdel.NodeControllerDelegate(), w.cfg.K0sDataDir); err != nil {
			return fmt.Errorf("unable to register 'controlnodes' controllers: %w", err)
		}
		// The controller-runtime start blocks until the context is cancelled.
		if err := mgr.Start(ctx); err != nil {
			return fmt.Errorf("unable to run controller-runtime manager for workers: %w", err)
		}
		return nil
	})
}
