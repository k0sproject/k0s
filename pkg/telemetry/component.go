// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"sync"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/segmentio/analytics-go"
	"github.com/sirupsen/logrus"
)

// Component is a telemetry component for k0s component manager
type Component struct {
	K0sVars           *config.CfgVars
	StorageType       v1beta1.StorageType
	KubeClientFactory kubeutil.ClientFactoryInterface

	log *logrus.Entry

	mu   sync.Mutex
	stop func()
}

var _ manager.Component = (*Component)(nil)
var _ manager.Reconciler = (*Component)(nil)

var interval = time.Minute * 10

// Init set up for external service clients (segment, k8s api)
func (c *Component) Init(context.Context) error {
	c.log = logrus.WithField("component", "telemetry")
	return nil
}

func (c *Component) Start(context.Context) error {
	return nil
}

func (c *Component) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stop != nil {
		c.stop()
		c.stop = nil
	}

	return nil
}

// Reconcile detects changes in configuration and applies them to the component
func (c *Component) Reconcile(_ context.Context, clusterCfg *v1beta1.ClusterConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !clusterCfg.Spec.Telemetry.IsEnabled() {
		if c.stop == nil {
			c.log.Debug("Telemetry remains disabled")
		} else {
			c.stop()
			c.stop = nil
		}

		return nil
	}

	if c.stop != nil {
		return nil // already running
	}

	clients, err := c.KubeClientFactory.GetClient()
	if err != nil {
		return err
	}

	c.stop = c.start(clients)

	return nil
}

func (c *Component) start(clients kubernetes.Interface) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		c.log.Info("Starting to collect telemetry")
		c.run(ctx, clients)
		c.log.Info("Stopped to collect telemetry")
	}()

	return func() { cancel(); <-done }
}

func (c *Component) run(ctx context.Context, clients kubernetes.Interface) {
	analyticsClient := analytics.New(segmentToken)
	defer func() {
		if err := analyticsClient.Close(); err != nil {
			c.log.WithError(err).Debug("Failed to close analytics client")
		}
	}()

	wait.UntilWithContext(ctx, func(ctx context.Context) {
		c.sendTelemetry(ctx, analyticsClient, clients)
	}, interval)
}
