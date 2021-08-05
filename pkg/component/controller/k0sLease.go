package controller

import (
	"context"
	"fmt"
	"os"

	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"

	"github.com/sirupsen/logrus"

	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"
)

// ControllerLease implements a component that manages a lease per controller.
// The per-controller leases are used to determine the amount of currently running controllers
type K0sLease struct {
	ClusterConfig     *v1beta1.ClusterConfig
	KubeClientFactory kubeutil.ClientFactoryInterface

	cancelCtx   context.Context
	cancelFunc  context.CancelFunc
	leaseCancel context.CancelFunc
}

// Init initializes the component needs
func (l *K0sLease) Init() error {
	return nil
}

// Run runs the leader elector to keep the lease object up-to-date.
func (l *K0sLease) Run() error {
	l.cancelCtx, l.cancelFunc = context.WithCancel(context.Background())
	log := logrus.WithFields(logrus.Fields{"component": "controllerlease"})
	client, err := l.KubeClientFactory.GetClient()
	if err != nil {
		return fmt.Errorf("can't create kubernetes rest client for lease pool: %v", err)
	}

	// hostname used to make the lease names be clear to which controller they belong to
	holderIdentity, err := os.Hostname()
	if err != nil {
		return nil
	}
	leaseID := fmt.Sprintf("k0s-ctrl-%s", holderIdentity)

	leasePool, err := leaderelection.NewLeasePool(client, leaseID, leaderelection.WithLogger(log))
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
			case <-l.cancelCtx.Done():
				return
			}
		}
	}()
	return nil
}

// Stop stops the component
func (l *K0sLease) Stop() error {
	if l.leaseCancel != nil {
		l.leaseCancel()
	}

	if l.cancelFunc != nil {
		l.cancelFunc()
	}
	return nil
}

// Healthy is a no-op healchcheck
func (l *K0sLease) Healthy() error { return nil }
