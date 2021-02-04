package server

import (
	"context"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"

	config "github.com/k0sproject/k0s/pkg/apis/v1beta1"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"
)

// ControllerLease implements a component that manages a lease per controller.
// The per-controller leases are used to determine the amount of currently running controllers
type ControllerLease struct {
	ClusterConfig     *config.ClusterConfig
	KubeClientFactory kubeutil.ClientFactory

	cancelCtx   context.Context
	cancelFunc  context.CancelFunc
	leaseCancel context.CancelFunc
}

// Init initializes the component needs
func (c *ControllerLease) Init() error {
	return nil
}

// Run runs the leader elector to keep the lease object up-to-date.
func (c *ControllerLease) Run() error {
	c.cancelCtx, c.cancelFunc = context.WithCancel(context.Background())
	log := logrus.WithFields(logrus.Fields{"component": "controllerlease"})
	client, err := c.KubeClientFactory.GetClient()
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

	c.leaseCancel = cancel

	go func() {
		for {
			select {
			case <-events.AcquiredLease:
				log.Info("acquired leader lease")
			case <-events.LostLease:
				log.Error("lost leader lease, this should not really happen!?!?!?")
			case <-c.cancelCtx.Done():
				return
			}
		}
	}()
	return nil
}

// Stop stops the component
func (c *ControllerLease) Stop() error {
	if c.leaseCancel != nil {
		c.leaseCancel()
	}

	if c.cancelFunc != nil {
		c.cancelFunc()
	}
	return nil
}

// Healthy is a no-op healchcheck
func (c *ControllerLease) Healthy() error { return nil }
