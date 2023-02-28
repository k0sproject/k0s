/*
Copyright 2020 k0s authors

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

package leaderelector

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/k0sproject/k0s/pkg/component/manager"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"
	"github.com/sirupsen/logrus"
)

type LeasePool struct {
	log *logrus.Entry

	stopCh            chan struct{}
	leaderStatus      atomic.Value
	kubeClientFactory kubeutil.ClientFactoryInterface
	leaseCancel       context.CancelFunc

	acquiredLeaseCallbacks []func()
	lostLeaseCallbacks     []func()
}

var _ Interface = (*LeasePool)(nil)
var _ manager.Component = (*LeasePool)(nil)

// NewLeasePool creates a new leader elector using a Kubernetes lease pool.
func NewLeasePool(kubeClientFactory kubeutil.ClientFactoryInterface) *LeasePool {
	d := atomic.Value{}
	d.Store(false)
	return &LeasePool{
		stopCh:            make(chan struct{}),
		kubeClientFactory: kubeClientFactory,
		log:               logrus.WithFields(logrus.Fields{"component": "poolleaderelector"}),
		leaderStatus:      d,
	}
}

func (l *LeasePool) Init(_ context.Context) error {
	return nil
}

func (l *LeasePool) Start(ctx context.Context) error {
	client, err := l.kubeClientFactory.GetClient()
	if err != nil {
		return fmt.Errorf("can't create kubernetes rest client for lease pool: %v", err)
	}
	leasePool, err := leaderelection.NewLeasePool(ctx, client, "k0s-endpoint-reconciler",
		leaderelection.WithLogger(l.log),
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
				l.log.Info("acquired leader lease")
				l.leaderStatus.Store(true)
				runCallbacks(l.acquiredLeaseCallbacks)
			case <-events.LostLease:
				l.log.Info("lost leader lease")
				l.leaderStatus.Store(false)
				runCallbacks(l.lostLeaseCallbacks)
			}
		}
	}()
	return nil
}

func runCallbacks(callbacks []func()) {
	for _, fn := range callbacks {
		if fn != nil {
			fn()
		}
	}
}

func (l *LeasePool) AddAcquiredLeaseCallback(fn func()) {
	l.acquiredLeaseCallbacks = append(l.acquiredLeaseCallbacks, fn)
}

func (l *LeasePool) AddLostLeaseCallback(fn func()) {
	l.lostLeaseCallbacks = append(l.lostLeaseCallbacks, fn)
}

func (l *LeasePool) Stop() error {
	if l.leaseCancel != nil {
		l.leaseCancel()
	}
	return nil
}

func (l *LeasePool) IsLeader() bool {
	return l.leaderStatus.Load().(bool)
}
