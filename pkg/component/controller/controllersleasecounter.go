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

	"github.com/sirupsen/logrus"
)

// K0sControllersLeaseCounter implements a component that manages a lease per controller.
// The per-controller leases are used to determine the amount of currently running controllers
type K0sControllersLeaseCounter struct {
	ClusterConfig     *v1beta1.ClusterConfig
	KubeClientFactory kubeutil.ClientFactoryInterface

	cancelFunc  context.CancelFunc
	leaseCancel context.CancelFunc

	subscribers []chan int
}

var _ manager.Component = (*K0sControllersLeaseCounter)(nil)

// Init initializes the component needs
func (l *K0sControllersLeaseCounter) Init(_ context.Context) error {
	l.subscribers = make([]chan int, 0)

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
	holderIdentity, err := node.GetNodename("")
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

	go l.runLeaseCounter(ctx)

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

// Check the numbers of controller every 10 secs and notify the subscribers
func (l *K0sControllersLeaseCounter) runLeaseCounter(ctx context.Context) {
	log := logrus.WithFields(logrus.Fields{"component": "controllerlease"})
	log.Debug("starting controller lease counter every 10 secs")
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info("stopping controller lease counter")
			return
		case <-ticker.C:
			log.Debug("counting controller lease holders")
			count, err := l.countLeaseHolders(ctx)
			if err != nil {
				log.Errorf("failed to count controller leases: %s", err)
			}
			l.notifySubscribers(count)
		}
	}
}

func (l *K0sControllersLeaseCounter) countLeaseHolders(ctx context.Context) (int, error) {
	client, err := l.KubeClientFactory.GetClient()
	if err != nil {
		return 0, err
	}

	return kubeutil.GetControlPlaneNodeCount(ctx, client)
}

// Notify the subscribers about the current controller count
func (l *K0sControllersLeaseCounter) notifySubscribers(count int) {
	log := logrus.WithFields(logrus.Fields{"component": "controllerlease"})
	log.Debugf("notifying subscribers (%d) about controller count: %d", len(l.subscribers), count)
	for _, ch := range l.subscribers {
		// Use non-blocking send to avoid blocking the loop
		select {
		case ch <- count:
		case <-time.After(5 * time.Second):
			log.Warn("timeout when sending count to subsrciber")
		}
	}
}

func (l *K0sControllersLeaseCounter) Subscribe() <-chan int {
	ch := make(chan int, 1)
	l.subscribers = append(l.subscribers, ch)
	return ch
}
