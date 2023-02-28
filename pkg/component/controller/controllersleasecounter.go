/*
Copyright 2021 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/manager"

	"github.com/sirupsen/logrus"

	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	nodeutil "k8s.io/kubernetes/pkg/util/node"
)

// K0sControllersLeaseCounter implements a component that manages a lease per controller.
// The per-controller leases are used to determine the amount of currently running controllers
type K0sControllersLeaseCounter struct {
	ClusterConfig     *v1beta1.ClusterConfig
	KubeClientFactory kubeutil.ClientFactoryInterface

	cancelFunc  context.CancelFunc
	leaseCancel context.CancelFunc
}

var _ manager.Component = (*K0sControllersLeaseCounter)(nil)

// Init initializes the component needs
func (l *K0sControllersLeaseCounter) Init(_ context.Context) error {
	return nil
}

// Run runs the leader elector to keep the lease object up-to-date.
func (l *K0sControllersLeaseCounter) Start(ctx context.Context) error {
	ctx, l.cancelFunc = context.WithCancel(ctx)
	log := logrus.WithFields(logrus.Fields{"component": "controllerlease"})
	client, err := l.KubeClientFactory.GetClient()
	if err != nil {
		return fmt.Errorf("can't create kubernetes rest client for lease pool: %v", err)
	}

	// hostname used to make the lease names be clear to which controller they belong to
	// follow kubelet convention for naming so we e.g. use lowercase hostname etc.
	holderIdentity, err := nodeutil.GetHostname("")
	if err != nil {
		return nil
	}
	leaseID := fmt.Sprintf("k0s-ctrl-%s", holderIdentity)

	leasePool, err := leaderelection.NewLeasePool(ctx, client, leaseID,
		leaderelection.WithLogger(log),
		leaderelection.WithContext(ctx))
	if err != nil {
		return err
	}
	events, cancel, err := leasePool.Watch()
	if err != nil {
		return err
	}

	l.leaseCancel = cancel

	go func() {
		for {
			select {
			case <-events.AcquiredLease:
				log.Info("acquired leader lease")
			case <-events.LostLease:
				log.Error("lost leader lease, this should not really happen!?!?!?")
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

// Stop stops the component
func (l *K0sControllersLeaseCounter) Stop() error {
	if l.leaseCancel != nil {
		l.leaseCancel()
	}

	if l.cancelFunc != nil {
		l.cancelFunc()
	}
	return nil
}
