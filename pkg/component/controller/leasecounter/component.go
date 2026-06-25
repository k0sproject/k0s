// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package leasecounter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/k0sproject/k0s/pkg/component/manager"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	corev1 "k8s.io/api/core/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/sirupsen/logrus"
)

// Implements a component that manages a lease per controller. The
// per-controller leases are used to determine the amount of currently running
// controllers
type Component struct {
	NodeName              apitypes.NodeName
	InvocationID          string
	KubeClientFactory     kubeutil.ClientFactoryInterface
	UpdateControllerCount func(uint)

	log  logrus.FieldLogger
	stop func()
}

var _ manager.Component = (*Component)(nil)

// Init initializes the component needs
func (c *Component) Init(context.Context) error {
	c.log = logrus.WithField("component", "controllerlease")
	return nil
}

// Run runs the leader elector to keep the lease object up-to-date.
func (c *Component) Start(context.Context) error {
	kubeClient, err := c.KubeClientFactory.GetClient()
	if err != nil {
		return fmt.Errorf("can't create kubernetes rest client for lease pool: %w", err)
	}

	// Use the node name to make the lease names be clear to which controller they belong to
	leaseName := "k0s-ctrl-" + string(c.NodeName)

	client, err := leaderelection.NewClient(&leaderelection.LeaseConfig{
		Namespace: corev1.NamespaceNodeLease,
		Name:      leaseName,
		Identity:  c.InvocationID,
		Client:    kubeClient.CoordinationV1(),
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	wg.Add(2)
	go func() { defer wg.Done(); c.runLeaderElection(ctx, client) }()
	go func() { defer wg.Done(); c.runLeaseCounter(ctx, kubeClient) }()
	c.stop = func() { cancel(); wg.Wait() }

	return nil
}

// Stop stops the component
func (c *Component) Stop() error {
	if c.stop != nil {
		c.stop()
	}
	return nil
}

func (c *Component) runLeaderElection(ctx context.Context, client *leaderelection.Client) {
	c.log.Info("Trying to acquire the controller lease")
	client.Run(ctx, func(status leaderelection.Status) {
		if status == leaderelection.StatusLeading {
			c.log.Info("Holding the controller lease")
		} else {
			c.log.Error("Lost the controller lease")
		}
	})
}

// Updates the number of active controller leases every 10 secs.
func (c *Component) runLeaseCounter(ctx context.Context, clients kubernetes.Interface) {
	c.log.Debug("Starting controller lease counter every 10 secs")

	wait.UntilWithContext(ctx, func(ctx context.Context) {
		c.log.Debug("Counting active controller leases")
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		count, err := kubeutil.CountActiveControllerLeases(ctx, clients)
		if err != nil {
			c.log.WithError(err).Error("Failed to count controller lease holders")
			return
		}

		c.UpdateControllerCount(count)
	}, 10*time.Second)
}
