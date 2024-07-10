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

package telemetry

import (
	"context"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	"k8s.io/client-go/kubernetes"

	"github.com/segmentio/analytics-go"
	"github.com/sirupsen/logrus"
)

// Component is a telemetry component for k0s component manager
type Component struct {
	clusterConfig     *v1beta1.ClusterConfig
	K0sVars           *config.CfgVars
	Version           string
	KubeClientFactory kubeutil.ClientFactoryInterface

	analyticsClient analytics.Client

	log    *logrus.Entry
	stopCh chan struct{}
}

var _ manager.Component = (*Component)(nil)
var _ manager.Reconciler = (*Component)(nil)

var interval = time.Minute * 10

// Init set up for external service clients (segment, k8s api)
func (c *Component) Init(_ context.Context) error {
	c.log = logrus.WithField("component", "telemetry")

	if segmentToken == "" {
		c.log.Info("no token, telemetry is disabled")
		return nil
	}

	c.analyticsClient = analytics.New(segmentToken)
	c.log.Info("segment client has been init")
	return nil
}

// Run runs work cycle
func (c *Component) Start(_ context.Context) error {
	return nil
}

// Run does nothing
func (c *Component) Stop() error {
	if segmentToken == "" {
		c.log.Info("no token, telemetry is disabled")
		return nil
	}
	if c.stopCh != nil {
		close(c.stopCh)
	}
	if c.analyticsClient != nil {
		_ = c.analyticsClient.Close()
	}
	return nil
}

// Reconcile detects changes in configuration and applies them to the component
func (c *Component) Reconcile(ctx context.Context, clusterCfg *v1beta1.ClusterConfig) error {
	logrus.Debug("reconcile method called for: Telemetry")
	if !clusterCfg.Spec.Telemetry.IsEnabled() {
		return c.Stop()
	}
	if c.stopCh != nil {
		// We must have the worker stuff already running, do nothing
		return nil
	}
	if segmentToken == "" {
		c.log.Info("no token, telemetry is disabled")
		return nil
	}
	c.clusterConfig = clusterCfg
	clients, err := c.KubeClientFactory.GetClient()
	if err != nil {
		return err
	}
	go c.run(ctx, clients)
	return nil
}

func (c *Component) run(ctx context.Context, clients kubernetes.Interface) {
	c.stopCh = make(chan struct{})
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.sendTelemetry(ctx, clients)
		case <-c.stopCh:
			return
		}
	}
}
