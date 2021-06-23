/*
Copyright 2021 k0s authors

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
	"time"

	"github.com/k0sproject/k0s/pkg/component"
	"github.com/k0sproject/k0s/pkg/k0scloudprovider"
)

type k0sCloudProvider struct {
	config         k0scloudprovider.Config
	stopCh         chan struct{}
	commandBuilder CommandBuilder
}

var _ component.Component = (*k0sCloudProvider)(nil)

// CommandBuilder allows for defining arbitrary functions that can
// create `Command` instances.
type CommandBuilder func() (k0scloudprovider.Command, error)

// NewK0sCloudProvider creates a new k0s cloud-provider using the default
// address collector and command.
func NewK0sCloudProvider(kubeConfigPath string, frequency time.Duration, port int) component.Component {
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
func newK0sCloudProvider(config k0scloudprovider.Config, cb CommandBuilder) component.Component {
	return &k0sCloudProvider{
		config:         config,
		stopCh:         make(chan struct{}),
		commandBuilder: cb,
	}
}

// Init in the k0s-cloud-provider intentionally does nothing.
func (c *k0sCloudProvider) Init() error {
	return nil
}

// Run will create a k0s-cloud-provider command, and run it on a goroutine.
// Failures to create this command will be returned as an error.
func (c *k0sCloudProvider) Run() error {
	command, err := c.commandBuilder()
	if err != nil {
		return err
	}

	go command(c.stopCh)

	return nil
}

// Stop will stop the k0s-cloud-provider command goroutine (if running)
func (c *k0sCloudProvider) Stop() error {
	close(c.stopCh)

	return nil
}

// Healthy in the k0s-cloud-provider intentionally does nothing.
func (c *k0sCloudProvider) Healthy() error {
	return nil
}
