/*
Copyright 2022 k0s authors

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
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/manager"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"
	"github.com/k0sproject/k0s/pkg/node"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/sirupsen/logrus"
)

// K0sControllersLeaseCounter implements a component that manages a lease per controller.
// The per-controller leases are used to determine the amount of currently running controllers
type K0sControllersLeaseCounter struct {
	InvocationID          string
	ClusterConfig         *v1beta1.ClusterConfig
	KubeClientFactory     kubeutil.ClientFactoryInterface
	UpdateControllerCount func(uint)

	cancelFunc context.CancelFunc
}

var _ manager.Component = (*K0sControllersLeaseCounter)(nil)

// Init initializes the component needs
func (l *K0sControllersLeaseCounter) Init(_ context.Context) error {
	return nil
}

// Run runs the leader elector to keep the lease object up-to-date.
func (l *K0sControllersLeaseCounter) Start(context.Context) error {
	log := logrus.WithFields(logrus.Fields{"component": "controllerlease"})
	client, err := l.KubeClientFactory.GetClient()
	if err != nil {
		return fmt.Errorf("can't create kubernetes rest client for lease pool: %w", err)
	}

	// hostname used to make the lease names be clear to which controller they belong to
	// follow kubelet convention for naming so we e.g. use lowercase hostname etc.
	nodeName, err := node.GetNodename("")
	if err != nil {
		return nil
	}
	leaseName := fmt.Sprintf("k0s-ctrl-%s", nodeName)

	leasePool, err := leaderelection.NewLeasePool(client, leaseName, l.InvocationID,
		leaderelection.WithLogger(log))
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	events, err := leasePool.Watch(ctx)
	if err != nil {
		cancel()
		return err
	}
	l.cancelFunc = cancel

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

	go l.runLeaseCounter(ctx, client)

	return nil
}

// Stop stops the component
func (l *K0sControllersLeaseCounter) Stop() error {
	if l.cancelFunc != nil {
		l.cancelFunc()
	}
	return nil
}

// Updates the number of active controller leases every 10 secs.
func (l *K0sControllersLeaseCounter) runLeaseCounter(ctx context.Context, clients kubernetes.Interface) {
	log := logrus.WithFields(logrus.Fields{"component": "controllerlease"})
	log.Debug("Starting controller lease counter every 10 secs")

	wait.UntilWithContext(ctx, func(ctx context.Context) {
		log.Debug("Counting active controller leases")
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		count, err := kubeutil.CountActiveControllerLeases(ctx, clients)
		if err != nil {
			log.WithError(err).Error("Failed to count controller lease holders")
			return
		}

		l.UpdateControllerCount(count)
	}, 10*time.Second)
}
