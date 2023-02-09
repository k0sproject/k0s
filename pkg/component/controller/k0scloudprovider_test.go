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
	"context"
	"sync"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/k0scloudprovider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
)

func EmptyAddressCollector(node *v1.Node) []v1.NodeAddress {
	return []v1.NodeAddress{}
}

// DummyCommand is a simple `k0scloudprovider.Command` that returns if the
// provided channel is closed, or after a 30s timeout.  The main motivation is
// to have a command that waits until completed, with notification.
func DummyCommand(wg *sync.WaitGroup, cancelled *bool) (k0scloudprovider.Command, error) {
	wg.Add(1)

	return func(stopCh <-chan struct{}) {
		defer wg.Done()

		t := time.NewTimer(30 * time.Second)
		defer t.Stop()

		select {
		case <-stopCh:
			*cancelled = true
		case <-t.C:
		}
	}, nil
}

// DummyCommandBuilder adapts `DummyCommand` to `CommandBuilder`
func DummyCommandBuilder(wg *sync.WaitGroup, cancelled *bool) CommandBuilder {
	return func() (k0scloudprovider.Command, error) {
		return DummyCommand(wg, cancelled)
	}
}

type K0sCloudProviderSuite struct {
	suite.Suite
	ccp       manager.Component
	cancelled bool
	wg        sync.WaitGroup
}

// SetupTest builds a makeshift k0s-cloud-provider configuration, and
// a dummy-command (no-op) per-test invocation.
func (suite *K0sCloudProviderSuite) SetupTest() {
	config := k0scloudprovider.Config{
		AddressCollector: EmptyAddressCollector,
		KubeConfig:       "/does/not/exist",
		UpdateFrequency:  1 * time.Second,
	}

	suite.ccp = newK0sCloudProvider(config, DummyCommandBuilder(&suite.wg, &suite.cancelled))
	assert.NotNil(suite.T(), suite.ccp)
}

// TestInit covers the `Init()` function.
func (suite *K0sCloudProviderSuite) TestInit() {
	assert.Nil(suite.T(), suite.ccp.Init(context.TODO()))
}

// TestRunStop covers the scenario of issuing a `Start()`, and ensuring
// that when `Stop()` is called, the underlying goroutine is cancelled.
// This is effectively testing the close-channel semantics baked into
// `Stop()`, without worrying about what was actually running.
func (suite *K0sCloudProviderSuite) TestRunStop() {
	ctx := context.TODO()
	assert.Nil(suite.T(), suite.ccp.Init(ctx))
	assert.Nil(suite.T(), suite.ccp.Start(ctx))

	// Ensures that the stopping mechanism actually closes the stop channel.
	assert.Nil(suite.T(), suite.ccp.Stop())
	suite.wg.Wait()

	assert.Equal(suite.T(), true, suite.cancelled)
}

// TestK0sCloudProviderTestSuite sets up the suite for testing.
func TestK0sCloudProviderTestSuite(t *testing.T) {
	suite.Run(t, new(K0sCloudProviderSuite))
}
