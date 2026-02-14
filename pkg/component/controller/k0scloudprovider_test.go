// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/k0scloudprovider"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
)

func EmptyAddressCollector(node *v1.Node) []v1.NodeAddress {
	return []v1.NodeAddress{}
}

// DummyCommand is a simple `k0scloudprovider.Command` that returns if the
// provided channel is closed, or after a 30s timeout.  The main motivation is
// to have a command that waits until completed, with notification.
func DummyCommand(wg *sync.WaitGroup, canceled *bool) (k0scloudprovider.Command, error) {
	wg.Add(1)

	return func(stopCh <-chan struct{}) {
		defer wg.Done()

		t := time.NewTimer(30 * time.Second)
		defer t.Stop()

		select {
		case <-stopCh:
			*canceled = true
		case <-t.C:
		}
	}, nil
}

// DummyCommandBuilder adapts `DummyCommand` to `CommandBuilder`
func DummyCommandBuilder(wg *sync.WaitGroup, canceled *bool) CommandBuilder {
	return func() (k0scloudprovider.Command, error) {
		return DummyCommand(wg, canceled)
	}
}

type K0sCloudProviderSuite struct {
	suite.Suite
	ccp      manager.Component
	canceled bool
	wg       sync.WaitGroup
}

// SetupTest builds a makeshift k0s-cloud-provider configuration, and
// a dummy-command (no-op) per-test invocation.
func (suite *K0sCloudProviderSuite) SetupTest() {
	config := k0scloudprovider.Config{
		AddressCollector: EmptyAddressCollector,
		KubeConfig:       "/does/not/exist",
		UpdateFrequency:  1 * time.Second,
	}

	suite.ccp = newK0sCloudProvider(config, DummyCommandBuilder(&suite.wg, &suite.canceled))
	suite.NotNil(suite.ccp)
}

// TestInit covers the `Init()` function.
func (suite *K0sCloudProviderSuite) TestInit() {
	suite.NoError(suite.ccp.Init(context.TODO()))
}

// TestRunStop covers the scenario of issuing a `Start()`, and ensuring
// that when `Stop()` is called, the underlying goroutine is canceled.
// This is effectively testing the close-channel semantics baked into
// `Stop()`, without worrying about what was actually running.
func (suite *K0sCloudProviderSuite) TestRunStop() {
	ctx := context.TODO()
	suite.NoError(suite.ccp.Init(ctx))
	suite.NoError(suite.ccp.Start(ctx))

	// Ensures that the stopping mechanism actually closes the stop channel.
	suite.NoError(suite.ccp.Stop())
	suite.wg.Wait()

	suite.True(suite.canceled)
}

// TestK0sCloudProviderTestSuite sets up the suite for testing.
func TestK0sCloudProviderTestSuite(t *testing.T) {
	suite.Run(t, new(K0sCloudProviderSuite))
}
