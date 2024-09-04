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

package leaderelector

import (
	"context"
	"fmt"
	"sync"

	"github.com/k0sproject/k0s/internal/sync/value"
	"github.com/k0sproject/k0s/pkg/component/manager"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	corev1 "k8s.io/api/core/v1"

	"github.com/sirupsen/logrus"
)

type LeasePool struct {
	log *logrus.Entry

	invocationID      string
	status            value.Latest[leaderelection.Status]
	kubeClientFactory kubeutil.ClientFactoryInterface
	stop              func()

	acquiredLeaseCallbacks []func()
	lostLeaseCallbacks     []func()
	name                   string
}

var (
	_ Interface         = (*LeasePool)(nil)
	_ manager.Component = (*LeasePool)(nil)
)

// NewLeasePool creates a new leader elector using a Kubernetes lease pool.
func NewLeasePool(invocationID string, kubeClientFactory kubeutil.ClientFactoryInterface, name string) *LeasePool {
	return &LeasePool{
		invocationID:      invocationID,
		kubeClientFactory: kubeClientFactory,
		log:               logrus.WithFields(logrus.Fields{"component": "poolleaderelector"}),
		name:              name,
	}
}

func (l *LeasePool) Init(_ context.Context) error {
	return nil
}

func (l *LeasePool) Start(context.Context) error {
	kubeClient, err := l.kubeClientFactory.GetClient()
	if err != nil {
		return fmt.Errorf("can't create kubernetes rest client for lease pool: %w", err)
	}

	client, err := leaderelection.NewClient(&leaderelection.LeaseConfig{
		Namespace: corev1.NamespaceNodeLease,
		Name:      l.name,
		Identity:  l.invocationID,
		Client:    kubeClient.CoordinationV1(),
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	wg.Add(2)
	go func() { defer wg.Done(); client.Run(ctx, l.status.Set) }()
	go func() { defer wg.Done(); l.invokeCallbacks(ctx.Done()) }()

	l.stop = func() { cancel(); wg.Wait() }
	return nil
}

func (l *LeasePool) invokeCallbacks(done <-chan struct{}) {
	var lastStatus leaderelection.Status

	for {
		status, statusChanged := l.status.Peek()

		if status != lastStatus {
			lastStatus = status
			if status == leaderelection.StatusLeading {
				l.log.Info("acquired leader lease")
				runCallbacks(l.acquiredLeaseCallbacks)
			} else {
				l.log.Info("lost leader lease")
				runCallbacks(l.lostLeaseCallbacks)
			}
		}

		select {
		case <-statusChanged:
		case <-done:
			l.log.Info("Lease pool is stopping")
			if status == leaderelection.StatusLeading {
				runCallbacks(l.lostLeaseCallbacks)
			}
			return
		}
	}
}

func runCallbacks(callbacks []func()) {
	for _, fn := range callbacks {
		if fn != nil {
			fn()
		}
	}
}

// Deprecated: Use [LeasePool.CurrentStatus] instead.
func (l *LeasePool) AddAcquiredLeaseCallback(fn func()) {
	l.acquiredLeaseCallbacks = append(l.acquiredLeaseCallbacks, fn)
}

// Deprecated: Use [LeasePool.CurrentStatus] instead.
func (l *LeasePool) AddLostLeaseCallback(fn func()) {
	l.lostLeaseCallbacks = append(l.lostLeaseCallbacks, fn)
}

func (l *LeasePool) Stop() error {
	if l.stop != nil {
		l.stop()
	}
	return nil
}

func (l *LeasePool) CurrentStatus() (leaderelection.Status, <-chan struct{}) {
	return l.status.Peek()
}

func (l *LeasePool) IsLeader() bool {
	status, _ := l.CurrentStatus()
	return status == leaderelection.StatusLeading
}
