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
	"errors"
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

	ctx, cancel := context.WithCancelCause(context.Background())
	var wg sync.WaitGroup

	wg.Add(2)
	go func() { defer wg.Done(); client.Run(ctx, l.status.Set) }()
	go func() { defer wg.Done(); l.invokeCallbacks(ctx) }()

	l.stop = func() { cancel(errors.New("lease pool is stopping")); wg.Wait() }
	return nil
}

func (l *LeasePool) invokeCallbacks(ctx context.Context) {
	leaderelection.RunLeaderTasks(ctx, l.status.Peek, func(ctx context.Context) {
		l.log.Info("acquired leader lease")
		runCallbacks(l.acquiredLeaseCallbacks)
		<-ctx.Done()
		l.log.Infof("lost leader lease (%v)", context.Cause(ctx))
		runCallbacks(l.lostLeaseCallbacks)
	})
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

// CurrentStatus is this lease pool's [leaderelection.StatusFunc].
func (l *LeasePool) CurrentStatus() (leaderelection.Status, <-chan struct{}) {
	return l.status.Peek()
}

// Deprecated: Use [LeasePool.CurrentStatus] instead.
func (l *LeasePool) IsLeader() bool {
	status, _ := l.CurrentStatus()
	return status == leaderelection.StatusLeading
}
