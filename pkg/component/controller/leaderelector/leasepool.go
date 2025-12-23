// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package leaderelector

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

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
	name              string
	client            leaseClient
	resume            value.Latest[time.Time]

	mu   sync.Mutex
	stop func()

	acquiredLeaseCallbacks []func()
	lostLeaseCallbacks     []func()
}

type leaseClient interface {
	Run(ctx context.Context, changed func(leaderelection.Status))
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

func (l *LeasePool) Init(context.Context) error {
	return nil
}

func (l *LeasePool) Start(context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.stop != nil {
		return nil
	}

	if l.client == nil {
		kubeClient, err := l.kubeClientFactory.GetClient()
		if err != nil {
			return fmt.Errorf("can't create kubernetes rest client for lease pool: %w", err)
		}

		l.client, err = leaderelection.NewClient(&leaderelection.LeaseConfig{
			Namespace: corev1.NamespaceNodeLease,
			Name:      l.name,
			Identity:  l.invocationID,
			Client:    kubeClient.CoordinationV1(),
		})
		if err != nil {
			return err
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				l.runClient(ctx.Done())
			}
		}
	}()

	l.stop = func() {
		cancel()
		<-done
	}

	return nil
}

func (l *LeasePool) runClient(stop <-chan struct{}) {
	resumeUpdated := l.waitForResume(stop)
	if resumeUpdated == nil {
		return
	}

	var reason error
	{
		var wg sync.WaitGroup

		ctx, cancel := context.WithCancelCause(context.Background())

		wg.Add(2)
		go func() { defer wg.Done(); l.client.Run(ctx, l.status.Set) }()
		go func() { defer wg.Done(); l.invokeCallbacks(ctx) }()
		defer func() { cancel(reason); wg.Wait() }()
	}

	select {
	case <-stop:
		reason = errors.New("lease pool is stopping")
	case <-resumeUpdated:
		reason = errors.New("lease pool is yielding")
	}
}

func (l *LeasePool) waitForResume(stop <-chan struct{}) <-chan struct{} {
	resume, resumeUpdated := l.resume.Peek()

	d := time.Until(resume)
	if d <= 0 {
		return resumeUpdated
	}
	timer := time.NewTimer(d)
	defer timer.Stop()

	l.log.Info("Yielding until ", resume)

	for {
		select {
		case <-stop:
			return nil
		case <-resumeUpdated:
			resume, resumeUpdated = l.resume.Peek()
			timer.Reset(time.Until(resume))
			l.log.Info("Yielding until ", resume)
		case <-timer.C:
			l.log.Info("Resuming operations")
			return resumeUpdated
		}
	}
}

func (l *LeasePool) YieldLease() {
	l.yieldLease(30 * time.Second)
}

func (l *LeasePool) yieldLease(duration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stop != nil {
		l.resume.Set(time.Now().Add(duration))
	}
}

func (l *LeasePool) invokeCallbacks(ctx context.Context) {
	leaderelection.RunLeaderTasks(ctx, l.status.Peek, func(leaderCtx context.Context) {
		l.log.Info("acquired leader lease")
		runCallbacks(l.acquiredLeaseCallbacks)
		<-leaderCtx.Done()
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
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stop != nil {
		l.stop()
		l.stop = nil
		l.resume.Set(time.Time{})
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
