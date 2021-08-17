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
package controller

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/component"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"
	"github.com/sirupsen/logrus"
)

// LeaderElector is the common leader elector component to manage each controller leader status
type LeaderElector interface {
	IsLeader() bool
	AddAcquiredLeaseCallback(fn func())
	AddLostLeaseCallback(fn func())
	component.Component
}

type leaderElector struct {
	ClusterConfig *v1beta1.ClusterConfig

	L *logrus.Entry

	stopCh            chan struct{}
	leaderStatus      atomic.Value
	kubeClientFactory kubeutil.ClientFactoryInterface
	leaseCancel       context.CancelFunc

	acquiredLeaseCallbacks []func()
	lostLeaseCallbacks     []func()
}

// NewLeaderElector creates new leader elector
func NewLeaderElector(c *v1beta1.ClusterConfig, kubeClientFactory kubeutil.ClientFactoryInterface) LeaderElector {
	d := atomic.Value{}
	d.Store(false)
	return &leaderElector{
		ClusterConfig:     c,
		stopCh:            make(chan struct{}),
		kubeClientFactory: kubeClientFactory,
		L:                 logrus.WithFields(logrus.Fields{"component": "endpointreconciler"}),
		leaderStatus:      d,
	}
}

func (l *leaderElector) Init() error {
	return nil
}

func (l *leaderElector) Run() error {
	client, err := l.kubeClientFactory.GetClient()
	if err != nil {
		return fmt.Errorf("can't create kubernetes rest client for lease pool: %v", err)
	}
	leasePool, err := leaderelection.NewLeasePool(client, "k0s-endpoint-reconciler", leaderelection.WithLogger(l.L))
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
				l.L.Info("acquired leader lease")
				l.leaderStatus.Store(true)
				runCallbacks(l.acquiredLeaseCallbacks)
			case <-events.LostLease:
				l.L.Info("lost leader lease")
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

func (l *leaderElector) AddAcquiredLeaseCallback(fn func()) {
	l.acquiredLeaseCallbacks = append(l.acquiredLeaseCallbacks, fn)
}

func (l *leaderElector) AddLostLeaseCallback(fn func()) {
	l.lostLeaseCallbacks = append(l.lostLeaseCallbacks, fn)
}

func (l *leaderElector) Stop() error {
	if l.leaseCancel != nil {
		l.leaseCancel()
	}
	return nil
}

// Reconcile detects changes in configuration and applies them to the component
func (l *leaderElector) Reconcile() error {
	logrus.Debug("reconcile method called for: leaderElector")
	return nil
}

func (l *leaderElector) IsLeader() bool {
	return l.leaderStatus.Load().(bool)
}

func (l *leaderElector) Healthy() error { return nil }
