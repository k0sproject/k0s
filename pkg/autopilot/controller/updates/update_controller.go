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

package updates

import (
	"context"
	"fmt"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	cr "sigs.k8s.io/controller-runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
)

type updateController struct {
	log           *logrus.Entry
	client        crcli.Client
	clientFactory apcli.FactoryInterface

	clusterID string

	updaters  map[string]updater
	parentCtx context.Context
}

func RegisterControllers(ctx context.Context, logger *logrus.Entry, mgr crman.Manager, clientFactory apcli.FactoryInterface, leaderMode bool, clusterID string) error {
	return cr.NewControllerManagedBy(mgr).
		Named("updater").
		For(&apv1beta2.UpdateConfig{}).
		Complete(
			&updateController{
				log:           logger.WithField("reconciler", "updater"),
				client:        mgr.GetClient(),
				clientFactory: clientFactory,
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
	if err := u.client.Get(ctx, crcli.ObjectKey{Name: "update-server-token", Namespace: "kube-system"}, tokenSecret); err != nil {
		u.log.Infof("unable to get update server token='%s': %v", req.NamespacedName, err)
	} else {
		token = string(tokenSecret.Data["token"])
	}

	u.log.Debugf("processing updater config '%s'", req.NamespacedName)

	// If the config is being deleted, stop the updater
	if !updaterConfig.DeletionTimestamp.IsZero() {
		u.log.Debugf("updater config '%s' is being deleted", req.NamespacedName)
		if updater, ok := u.updaters[req.NamespacedName.String()]; ok {
			u.log.Debugf("stopping existing updater for '%s'", req.NamespacedName)
			updater.Stop()
			delete(u.updaters, req.NamespacedName.String())
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
	if updater, ok := u.updaters[req.NamespacedName.String()]; ok {
		// Check if there's been updates to the config, if so re-create the updater
		if updater.Config() == nil || updater.Config().ObjectMeta.ResourceVersion != updaterConfig.ResourceVersion {
			u.log.Debugf("updater config '%s' has been updated, re-creating updater", req.NamespacedName)
			updater.Stop()
			delete(u.updaters, req.NamespacedName.String())
		}
	}
	u.log.Debugf("creating new updater for '%s'", req.NamespacedName)
	// Create new updater
	updater, err := newUpdater(u.parentCtx, *updaterConfig, u.client, u.clientFactory, u.clusterID, token)
	if err != nil {
		u.log.Errorf("failed to create updater for '%s': %s", req.NamespacedName, err)
		return cr.Result{}, err
	}
	u.updaters[req.NamespacedName.String()] = updater
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
