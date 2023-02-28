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

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2"
	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	cr "sigs.k8s.io/controller-runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
)

type updateController struct {
	log           *logrus.Entry
	client        crcli.Client
	clientFactory apcli.FactoryInterface

	clusterID string

	updater *updater
}

func RegisterControllers(ctx context.Context, logger *logrus.Entry, mgr crman.Manager, clientFactory apcli.FactoryInterface, leaderMode bool, clusterID string) error {
	return cr.NewControllerManagedBy(mgr).
		For(&apv1beta2.UpdateConfig{}).
		Complete(
			&updateController{
				log:           logger.WithField("reconciler", "updater"),
				client:        mgr.GetClient(),
				clientFactory: clientFactory,
				clusterID:     clusterID,
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
		u.log.Errorf("unable to get plan='%s': %v", req.NamespacedName, err)
	} else {
		token = string(tokenSecret.Data["token"])
	}

	u.log.Infof("processing updater config '%s'", req.NamespacedName)

	if u.updater == nil {
		updater, err := newUpdater(ctx, *updaterConfig, u.client, u.clusterID, token)
		if err != nil {
			return cr.Result{}, err
		}
		u.updater = updater
		u.updater.Run()
	}

	return cr.Result{}, nil
}
