//go:build unix

// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	aproot "github.com/k0sproject/k0s/pkg/autopilot/controller/root"
	"github.com/k0sproject/k0s/pkg/autopilot/controller/signal"
	apsigk0s "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/k0s"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	corev1 "k8s.io/api/core/v1"
	extensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	cr "sigs.k8s.io/controller-runtime"
	crcache "sigs.k8s.io/controller-runtime/pkg/cache"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
	crmetricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	crwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/sirupsen/logrus"
)

type rootWorker struct {
	cfg           aproot.RootConfig
	log           *logrus.Entry
	clientFactory kubernetes.ClientFactoryInterface
}

var _ aproot.Root = (*rootWorker)(nil)

// NewRootWorker builds a root for autopilot "worker" operations.
func NewRootWorker(cfg aproot.RootConfig, logger *logrus.Entry, cf kubernetes.ClientFactoryInterface) (aproot.Root, error) {
	c := &rootWorker{
		cfg:           cfg,
		log:           logger,
		clientFactory: cf,
	}

	return c, nil
}

func (w *rootWorker) Run(ctx context.Context) error {
	if w.cfg.NodeName == "" {
		return errors.New("node name is required")
	}

	logger := w.log

	// The manager's field indexer on plans needs the CRD to be served, and
	// controllers may deploy it after this worker has joined the cluster.
	if err := w.waitForPlansCRD(ctx); err != nil {
		return fmt.Errorf("while waiting for autopilot 'plans' CRD: %w", err)
	}

	clusterID, err := w.getClusterID(ctx)
	if err != nil {
		return err
	}

	restConfig, err := w.clientFactory.GetRESTConfig()
	if err != nil {
		return err
	}

	mgr, err := cr.NewManager(restConfig, crman.Options{
		Scheme: scheme,
		WebhookServer: crwebhook.NewServer(crwebhook.Options{
			Port: w.cfg.ManagerPort,
		}),
		Metrics: crmetricsserver.Options{
			BindAddress: w.cfg.MetricsBindAddr,
		},
		HealthProbeBindAddress: w.cfg.HealthProbeBindAddr,
		// The worker controllers only ever reconcile the worker's own Node. Restrict
		// the Node informer to it, so that the worker doesn't receive every Node
		// update in the cluster, which dominates its traffic on large clusters.
		Cache: crcache.Options{
			ByObject: map[crcli.Object]crcache.ByObject{
				&corev1.Node{}: {Field: fields.OneTermEqualSelector(metav1.ObjectNameField, string(w.cfg.NodeName))},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create controller manager: %w", err)
	}

	var restartTracker apsigk0s.RestartTracker
	if err := signal.RegisterControllers(ctx, logger, mgr, apdel.NodeControllerDelegate(), w.cfg.NodeName, &restartTracker, w.cfg.K0sDataDir, true, clusterID, leaderelection.StatusPending); err != nil {
		return fmt.Errorf("unable to register signal controllers: %w", err)
	}

	// The controller-runtime start blocks until the context is canceled.
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("unable to run controller-runtime manager for workers: %w", err)
	}
	return nil
}

// Blocks until the plans CRD is established. Keeps watching across errors,
// as workers may come up long before the controllers do.
func (w *rootWorker) waitForPlansCRD(ctx context.Context) error {
	client, err := w.clientFactory.GetAPIExtensionsClient()
	if err != nil {
		return err
	}

	return watch.CRDs(client.ApiextensionsV1().CustomResourceDefinitions()).
		WithObjectName("plans."+apv1beta2.GroupName).
		WithErrorCallback(func(err error) (time.Duration, error) {
			if retryAfter, e := watch.IsRetryable(err); e == nil {
				w.log.WithError(err).Infof(
					"Transient error while watching for autopilot 'plans' CRD, starting over after %s ...",
					retryAfter,
				)
				return retryAfter, nil
			}

			retryAfter := 10 * time.Second
			w.log.WithError(err).Errorf(
				"Failed to watch for autopilot 'plans' CRD, starting over after %s ...",
				retryAfter,
			)
			return retryAfter, nil
		}).
		Until(ctx, func(crd *extensionsv1.CustomResourceDefinition) (bool, error) {
			for _, cond := range crd.Status.Conditions {
				if cond.Type == extensionsv1.Established {
					return cond.Status == extensionsv1.ConditionTrue, nil
				}
			}

			return false, nil
		})
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
