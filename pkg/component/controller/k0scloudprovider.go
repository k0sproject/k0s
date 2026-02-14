// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"time"

	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/k0scloudprovider"
)

type K0sCloudProvider struct {
	config         k0scloudprovider.Config
	stopCh         chan struct{}
	commandBuilder CommandBuilder
}

var _ manager.Component = (*K0sCloudProvider)(nil)

// CommandBuilder allows for defining arbitrary functions that can
// create `Command` instances.
type CommandBuilder func() (k0scloudprovider.Command, error)

// NewK0sCloudProvider creates a new k0s cloud-provider using the default
// address collector and command.
func NewK0sCloudProvider(kubeConfigPath string, frequency time.Duration, port int) *K0sCloudProvider {
	config := k0scloudprovider.Config{
		AddressCollector: k0scloudprovider.DefaultAddressCollector(),
		KubeConfig:       kubeConfigPath,
		UpdateFrequency:  frequency,
		BindPort:         port,
	}

	return newK0sCloudProvider(config, func() (k0scloudprovider.Command, error) {
		return k0scloudprovider.NewCommand(config)
	})
}

// newK0sCloudProvider is a helper for creating specialized k0s-cloud-provider
// instances that can be used for testing.
func newK0sCloudProvider(config k0scloudprovider.Config, cb CommandBuilder) *K0sCloudProvider {
	return &K0sCloudProvider{
		config:         config,
		stopCh:         make(chan struct{}),
		commandBuilder: cb,
	}
}

// Init in the k0s-cloud-provider intentionally does nothing.
func (c *K0sCloudProvider) Init(_ context.Context) error {
	return nil
}

// Run will create a k0s-cloud-provider command, and run it on a goroutine.
// Failures to create this command will be returned as an error.
func (c *K0sCloudProvider) Start(_ context.Context) error {
	command, err := c.commandBuilder()
	if err != nil {
		return err
	}

	go command(c.stopCh)

	return nil
}

// Stop will stop the k0s-cloud-provider command goroutine (if running)
func (c *K0sCloudProvider) Stop() error {
	close(c.stopCh)

	return nil
}
