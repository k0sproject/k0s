//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package updates

import (
	"context"
	"fmt"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sirupsen/logrus"
	cr "sigs.k8s.io/controller-runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
)

type updateController struct {
	log           *logrus.Entry
	client        crcli.Client
	clientFactory apcli.FactoryInterface
	collector     *ClusterInfoCollector

	clusterID string

	updaters  map[string]updater
	parentCtx context.Context
}

func RegisterControllers(ctx context.Context, logger *logrus.Entry, mgr crman.Manager, clientFactory apcli.FactoryInterface, collector *ClusterInfoCollector, leaderMode bool, clusterID string) error {
	return cr.NewControllerManagedBy(mgr).
		Named("updater").
		For(&apv1beta2.UpdateConfig{}).
		Complete(
			&updateController{
				log:           logger.WithField("reconciler", "updater"),
				client:        mgr.GetClient(),
				clientFactory: clientFactory,
				collector:     collector,
				clusterID:     clusterID,
				updaters:      make(map[string]updater),
				parentCtx:     ctx,
			},
		)
}

func (u *updateController) Reconcile(ctx context.Context, req cr.Request) (cr.Result, error) {
	updaterConfig := &apv1beta2.UpdateConfig{}
	if err := u.client.Get(ctx, req.NamespacedName, updaterConfig); err != nil {
		return cr.Result{}, fmt.Errorf("unable to get plan='%s': %w", req.NamespacedName, err)
	}

	var token string
	tokenSecret := &corev1.Secret{}
	if err := u.client.Get(ctx, crcli.ObjectKey{Name: "update-server-token", Namespace: metav1.NamespaceSystem}, tokenSecret); err != nil {
		u.log.Infof("unable to get update server token='%s': %v", req.NamespacedName, err)
	} else {
		token = string(tokenSecret.Data["token"])
	}

	u.log.Debugf("processing updater config '%s'", req.NamespacedName)

	// If the config is being deleted, stop the updater
	if !updaterConfig.DeletionTimestamp.IsZero() {
		u.log.Debugf("updater config '%s' is being deleted", req.NamespacedName)
		if updater, ok := u.updaters[req.String()]; ok {
			u.log.Debugf("stopping existing updater for '%s'", req.NamespacedName)
			updater.Stop()
			delete(u.updaters, req.String())
		}
		// Remove finalizer
		controllerutil.RemoveFinalizer(updaterConfig, apv1beta2.UpdateConfigFinalizer)
		if err := u.client.Update(ctx, updaterConfig); err != nil {
			return cr.Result{}, err
		}
		return cr.Result{}, nil
	}
	u.log.Debugf("checking if there's an existing updater for '%s'", req.NamespacedName)
	// Find the updater for this config if exists
	if updater, ok := u.updaters[req.String()]; ok {
		// Check if there's been updates to the config, if so re-create the updater
		if updater.Config() == nil || updater.Config().ResourceVersion != updaterConfig.ResourceVersion {
			u.log.Debugf("updater config '%s' has been updated, re-creating updater", req.NamespacedName)
			updater.Stop()
			delete(u.updaters, req.String())
		}
	}
	u.log.Debugf("creating new updater for '%s'", req.NamespacedName)
	// Create new updater
	updater, err := newUpdater(u.parentCtx, *updaterConfig, u.client, u.clientFactory, u.clusterID, u.collector, token)
	if err != nil {
		u.log.Errorf("failed to create updater for '%s': %s", req.NamespacedName, err)
		return cr.Result{}, err
	}
	u.updaters[req.String()] = updater
	if err := updater.Run(); err != nil {
		return cr.Result{}, err
	}

	// Add finalizer if not present
	controllerutil.AddFinalizer(updaterConfig, apv1beta2.UpdateConfigFinalizer)
	if err := u.client.Update(ctx, updaterConfig); err != nil {
		return cr.Result{}, err
	}

	return cr.Result{}, nil
}
